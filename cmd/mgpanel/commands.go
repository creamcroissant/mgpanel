package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/bootstrap"
	"github.com/creamcroissant/mgpanel/internal/config"
	"github.com/creamcroissant/mgpanel/internal/job"
	"github.com/creamcroissant/mgpanel/internal/migrations"
	"github.com/creamcroissant/mgpanel/internal/notifier"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/repository/sqlite"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
	"github.com/spf13/cobra"
)

func init() {
	// Migrate
	var migrateStatus bool
	var migrateRollback bool
	var migrateCmd = &cobra.Command{
		Use:   "migrate [up|down|status]",
		Short: "Database migration management",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
			if err != nil {
				return err
			}
			resolvedDBPath, err := bootstrap.ResolveSQLitePath(cfg.DB.Path)
			if err != nil {
				return err
			}
			cfg.DB.Path = resolvedDBPath
			db, err := bootstrap.OpenSQLite(cfg.DB.Path)
			if err != nil {
				return err
			}
			fmt.Printf("Using DB path: %s\n", cfg.DB.Path)
			defer db.Close()

			if migrateStatus {
				return migrations.Status(db)
			}
			if migrateRollback {
				return migrations.Down(db)
			}

			action := "up"
			if len(args) > 0 {
				action = args[0]
			}

			switch action {
			case "up":
				return migrations.Up(db)
			case "down":
				return migrations.Down(db)
			case "status":
				return migrations.Status(db)
			default:
				return fmt.Errorf("unknown migrate action %q", action)
			}
		},
	}
	migrateCmd.Flags().BoolVar(&migrateStatus, "status", false, "Show migration status")
	migrateCmd.Flags().BoolVar(&migrateRollback, "rollback", false, "Rollback the last migration")
	rootCmd.AddCommand(migrateCmd)

	// Backup
	var backupOutput string
	var backupCompress bool
	var backupCmd = &cobra.Command{
		Use:   "backup",
		Short: "Backup database",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
			if err != nil {
				return err
			}
			resolvedDBPath, err := bootstrap.ResolveSQLitePath(cfg.DB.Path)
			if err != nil {
				return err
			}
			cfg.DB.Path = resolvedDBPath
			target := backupOutput
			if target == "" {
				backupDir := "data/backups"
				if err := os.MkdirAll(backupDir, 0755); err != nil {
					return fmt.Errorf("create backup dir: %w", err)
				}
				ext := ".db"
				if backupCompress {
					ext += ".gz"
				}
				filename := fmt.Sprintf("mgpanel_%s%s", time.Now().Format("20060102_150405"), ext)
				target = filepath.Join(backupDir, filename)
			}

			db, err := bootstrap.OpenSQLite(cfg.DB.Path)
			if err != nil {
				return err
			}
			defer db.Close()

			tempFile := target
			if backupCompress {
				if strings.HasSuffix(target, ".gz") {
					tempFile = strings.TrimSuffix(target, ".gz")
				} else {
					tempFile = target + ".tmp"
				}
			}

			if _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", tempFile)); err != nil {
				return fmt.Errorf("sqlite vacuum into: %w", err)
			}

			if backupCompress {
				if err := compressFile(tempFile, target); err != nil {
					if rmErr := os.Remove(tempFile); rmErr != nil {
						slog.Warn("failed to remove temp backup file", "error", rmErr)
					}
					return err
				}
				if err := os.Remove(tempFile); err != nil {
					slog.Warn("failed to remove temp backup file", "error", err)
				}
			}

			fmt.Printf("Backup created at %s\n", target)
			return nil
		},
	}
	backupCmd.Flags().StringVar(&backupOutput, "output", "", "Output file path")
	backupCmd.Flags().BoolVar(&backupCompress, "compress", false, "Compress output with gzip")
	rootCmd.AddCommand(backupCmd)

	// Restore
	var restoreCmd = &cobra.Command{
		Use:   "restore <backup-file>",
		Short: "Restore database from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
			if err != nil {
				return err
			}
			resolvedDBPath, err := bootstrap.ResolveSQLitePath(cfg.DB.Path)
			if err != nil {
				return err
			}
			cfg.DB.Path = resolvedDBPath
			backupPath := args[0]
			if _, err := os.Stat(backupPath); err != nil {
				return fmt.Errorf("backup file not found: %w", err)
			}

			dbPath := cfg.DB.Path
			// Auto-backup before restore
			if _, err := os.Stat(dbPath); err == nil {
				bakPath := dbPath + ".pre_restore_" + time.Now().Format("20060102_150405")
				if err := copyFile(dbPath, bakPath); err != nil {
					return fmt.Errorf("failed to backup current db: %w", err)
				}
				fmt.Printf("Current database backed up to %s\n", bakPath)
			}

			isGzip := strings.HasSuffix(backupPath, ".gz")
			sourceFile := backupPath

			if isGzip {
				tempSource := dbPath + ".restoring"
				defer os.Remove(tempSource)
				if err := decompressFile(backupPath, tempSource); err != nil {
					return fmt.Errorf("decompress failed: %w", err)
				}
				sourceFile = tempSource
			}

			if err := copyFile(sourceFile, dbPath); err != nil {
				return fmt.Errorf("restore failed: %w", err)
			}

			fmt.Println("Database restored successfully.")
			return nil
		},
	}
	rootCmd.AddCommand(restoreCmd)

	// User
	var userCmd = &cobra.Command{
		Use:   "user",
		Short: "User management",
	}
	userCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runUserList(store)
		},
	})

	var createUserEmail, createUserPassword string
	var createUserAdmin bool
	var createUserCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if createUserEmail == "" || createUserPassword == "" {
				return fmt.Errorf("email and password are required")
			}
			store, cfg, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runUserCreate(store, cfg, createUserEmail, createUserPassword, createUserAdmin)
		},
	}
	createUserCmd.Flags().StringVar(&createUserEmail, "email", "", "User email")
	createUserCmd.Flags().StringVar(&createUserPassword, "password", "", "User password")
	createUserCmd.Flags().BoolVar(&createUserAdmin, "admin", false, "Set as admin")
	userCmd.AddCommand(createUserCmd)

	var resetUserEmail, resetUserPassword string
	var resetPasswordCmd = &cobra.Command{
		Use:   "reset-password",
		Short: "Reset user password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if resetUserEmail == "" || resetUserPassword == "" {
				return fmt.Errorf("email and password are required")
			}
			store, cfg, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runUserResetPassword(store, cfg, resetUserEmail, resetUserPassword)
		},
	}
	resetPasswordCmd.Flags().StringVar(&resetUserEmail, "email", "", "User email")
	resetPasswordCmd.Flags().StringVar(&resetUserPassword, "password", "", "New password")
	userCmd.AddCommand(resetPasswordCmd)

	userCmd.AddCommand(&cobra.Command{
		Use:   "disable <email>",
		Short: "Disable a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runUserStatus(store, args[0], 0)
		},
	})

	userCmd.AddCommand(&cobra.Command{
		Use:   "enable <email>",
		Short: "Enable a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runUserStatus(store, args[0], 1)
		},
	})
	rootCmd.AddCommand(userCmd)

	// Config
	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Short: "Get configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runConfigGet(store, args[0])
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runConfigSet(store, args[0], args[1])
		},
	})
	rootCmd.AddCommand(configCmd)

	// Job
	var jobCmd = &cobra.Command{
		Use:   "job",
		Short: "Job management",
	}
	jobCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available jobs",
		Run: func(cmd *cobra.Command, args []string) {
			jobs := getJobs(nil) // store not needed for list keys
			fmt.Println("Available jobs:")
			for name := range jobs {
				fmt.Println("- " + name)
			}
		},
	})
	jobCmd.AddCommand(&cobra.Command{
		Use:   "run <name>",
		Short: "Run a job manually",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			jobs := getJobs(store)
			name := args[0]
			j, ok := jobs[name]
			if !ok {
				return fmt.Errorf("unknown job %q", name)
			}
			fmt.Printf("Running job %s...\n", name)
			runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := j.Run(runCtx); err != nil {
				return fmt.Errorf("job run failed: %w", err)
			}
			fmt.Println("Job completed successfully.")
			return nil
		},
	})
	rootCmd.AddCommand(jobCmd)

	// Version
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("MGPanel Go Edition %s\n", Version)
			fmt.Printf("Commit: %s\n", Commit)
			fmt.Printf("Build Time: %s\n", BuildTime)
		},
	}
	rootCmd.AddCommand(versionCmd)
}

// Helper functions

func getStore() (*sqlite.Store, *config.Config, func(), error) {
	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
	if err != nil {
		return nil, nil, nil, err
	}
	resolvedDBPath, err := bootstrap.ResolveSQLitePath(cfg.DB.Path)
	if err != nil {
		return nil, nil, nil, err
	}
	cfg.DB.Path = resolvedDBPath
	db, err := bootstrap.OpenSQLite(cfg.DB.Path)
	if err != nil {
		return nil, nil, nil, err
	}
	return sqlite.NewStore(db), cfg, func() { db.Close() }, nil
}

func runUserList(store *sqlite.Store) error {
	ctx := context.Background()
	users, err := store.Users().Search(ctx, repository.UserSearchFilter{Limit: 100})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tEmail\tAdmin\tStatus")
	for _, u := range users {
		fmt.Fprintf(w, "%d\t%s\t%v\t%d\n", u.ID, u.Email, u.IsAdmin, u.Status)
	}
	if err := w.Flush(); err != nil {
		slog.Warn("flush output failed", "error", err)
	}
	return nil
}

func runUserCreate(store *sqlite.Store, cfg *config.Config, email, password string, isAdmin bool) error {
	hasher, err := hash.NewBcryptHasher(cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	hashed, err := hasher.Hash(password)
	if err != nil {
		return err
	}

	user := &repository.User{
		Email:        email,
		Password:     hashed,
		IsAdmin:      isAdmin,
		Status:       1,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
		UUID:         fmt.Sprintf("cli-created-%d", time.Now().UnixNano()),
		Token:        fmt.Sprintf("cli-token-%d", time.Now().UnixNano()),
		PasswordAlgo: "bcrypt",
	}

	if _, err := store.Users().Create(context.Background(), user); err != nil {
		return fmt.Errorf("create user failed: %w", err)
	}
	fmt.Printf("User %s created.\n", email)
	return nil
}

func runUserResetPassword(store *sqlite.Store, cfg *config.Config, email, password string) error {
	ctx := context.Background()
	user, err := store.Users().FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	hasher, err := hash.NewBcryptHasher(cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	hashed, err := hasher.Hash(password)
	if err != nil {
		return err
	}

	user.Password = hashed
	user.PasswordAlgo = "bcrypt"
	user.UpdatedAt = time.Now().Unix()

	if err := store.Users().Save(ctx, user); err != nil {
		return fmt.Errorf("save user failed: %w", err)
	}
	fmt.Printf("Password reset for %s.\n", email)
	return nil
}

func runUserStatus(store *sqlite.Store, identifier string, status int) error {
	ctx := context.Background()
	user, err := store.Users().FindByEmail(ctx, identifier)
	if err != nil {
		return fmt.Errorf("find user failed: %w", err)
	}

	user.Status = status
	user.UpdatedAt = time.Now().Unix()

	if err := store.Users().Save(ctx, user); err != nil {
		return fmt.Errorf("update user failed: %w", err)
	}
	action := "enabled"
	if status == 0 {
		action = "disabled"
	}
	fmt.Printf("User %s %s.\n", identifier, action)
	return nil
}

func runConfigGet(store *sqlite.Store, key string) error {
	ctx := context.Background()
	setting, err := store.Settings().Get(ctx, key)
	if err != nil {
		return fmt.Errorf("get config failed: %w", err)
	}
	if setting == nil {
		fmt.Println("<nil>")
	} else {
		fmt.Println(setting.Value)
	}
	return nil
}

func runConfigSet(store *sqlite.Store, key, value string) error {
	ctx := context.Background()
	setting := &repository.Setting{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now().Unix(),
	}
	if err := store.Settings().Upsert(ctx, setting); err != nil {
		return fmt.Errorf("set config failed: %w", err)
	}
	fmt.Printf("Config %s set.\n", key)
	return nil
}

func getJobs(store *sqlite.Store) map[string]job.Runnable {
	trafficQueue := async.NewTrafficQueue()
	notificationQueue := async.NewNotificationQueue()
	statAccumulator := job.NewStatUserAccumulator()

	// Store might be nil if just listing
	var trafficSvc service.ServerTrafficService
	var statRepo repository.StatUserRepository

	if store != nil {
		trafficSvc = service.NewServerTrafficService(store.Users(), nil)
		statRepo = store.StatUsers()
	}

	notifierSvc := notifier.NewLoggerService(nil)

	return map[string]job.Runnable{
		"traffic.fetch":   job.NewTrafficFetchJob(trafficQueue, trafficSvc, nil),
		"stat.user":       job.NewStatUserJob(statAccumulator, statRepo, nil),
		"notify.email":    job.NewSendEmailJob(notificationQueue, notifierSvc, nil),
		"notify.telegram": job.NewSendTelegramJob(notificationQueue, notifierSvc, nil),
	}
}

// File utils
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func compressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	gw := gzip.NewWriter(out)

	if _, err := io.Copy(gw, in); err != nil {
		gw.Close()
		out.Close()
		return err
	}

	if err := gw.Close(); err != nil {
		out.Close()
		return fmt.Errorf("close gzip writer: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close output file: %w", err)
	}
	return nil
}

func decompressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	gr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		gr.Close()
		return err
	}

	if _, err := io.Copy(out, gr); err != nil {
		gr.Close()
		out.Close()
		return err
	}
	if err := gr.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
