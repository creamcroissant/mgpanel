package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/repository/sqlite"
	"github.com/spf13/cobra"
)

func init() {
	var statCmd = &cobra.Command{
		Use:   "stat",
		Short: "Statistics and analytics commands",
		Long:  `View traffic statistics for users and nodes.`,
	}

	// stat overview
	var overviewCmd = &cobra.Command{
		Use:   "overview",
		Short: "Show overall system statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runStatOverview(store)
		},
	}
	statCmd.AddCommand(overviewCmd)

	// stat user <user_id>
	var userStatDays int
	var userStatCmd = &cobra.Command{
		Use:   "user <user_id>",
		Short: "Show traffic statistics for a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			userID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}
			return runStatUser(store, userID, userStatDays)
		},
	}
	userStatCmd.Flags().IntVarP(&userStatDays, "days", "d", 30, "Number of days to show")
	statCmd.AddCommand(userStatCmd)

	// stat top
	var topLimit int
	var topDays int
	var topCmd = &cobra.Command{
		Use:   "top",
		Short: "Show top users by traffic usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runStatTop(store, topLimit, topDays)
		},
	}
	topCmd.Flags().IntVarP(&topLimit, "limit", "l", 10, "Number of top users to show")
	topCmd.Flags().IntVarP(&topDays, "days", "d", 30, "Period in days")
	statCmd.AddCommand(topCmd)

	// stat traffic
	var trafficDays int
	var trafficCmd = &cobra.Command{
		Use:   "traffic",
		Short: "Show total traffic statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, cleanup, err := getStore()
			if err != nil {
				return err
			}
			defer cleanup()
			return runStatTraffic(store, trafficDays)
		},
	}
	trafficCmd.Flags().IntVarP(&trafficDays, "days", "d", 30, "Period in days")
	statCmd.AddCommand(trafficCmd)

	rootCmd.AddCommand(statCmd)
}

func runStatOverview(store *sqlite.Store) error {
	ctx := context.Background()
	now := time.Now()

	// User counts
	totalUsers, err := store.Users().Count(ctx)
	if err != nil {
		return err
	}
	activeUsers, err := store.Users().CountActive(ctx, now.Unix())
	if err != nil {
		return err
	}

	// Monthly new users
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	newUsers, err := store.Users().CountCreatedBetween(ctx, monthStart.Unix(), now.Unix())
	if err != nil {
		return err
	}

	// Node counts
	allNodes, err := store.Servers().ListAll(ctx)
	if err != nil {
		return err
	}

	var onlineNodes int
	for _, n := range allNodes {
		if now.Unix()-n.LastHeartbeatAt < 120 {
			onlineNodes++
		}
	}

	// Monthly traffic (from stat_users)
	filter := repository.StatUserSumFilter{
		RecordType: 1, // daily
		StartAt:    monthStart.Unix(),
		EndAt:      now.Unix(),
	}
	trafficSum, err := store.StatUsers().SumByRange(ctx, filter)
	if err != nil {
		return err
	}

	fmt.Println("System Overview")
	fmt.Println("===============")
	fmt.Println()
	fmt.Println("Users:")
	fmt.Printf("  Total:     %d\n", totalUsers)
	fmt.Printf("  Active:    %d\n", activeUsers)
	fmt.Printf("  New (MTD): %d\n", newUsers)
	fmt.Println()
	fmt.Println("Nodes:")
	fmt.Printf("  Total:     %d\n", len(allNodes))
	fmt.Printf("  Online:    %d\n", onlineNodes)
	fmt.Println()
	fmt.Println("Traffic (MTD):")
	fmt.Printf("  Upload:    %s\n", formatBytes(trafficSum.Upload))
	fmt.Printf("  Download:  %s\n", formatBytes(trafficSum.Download))
	fmt.Printf("  Total:     %s\n", formatBytes(trafficSum.Upload+trafficSum.Download))

	return nil
}

func runStatUser(store *sqlite.Store, userID int64, days int) error {
	ctx := context.Background()

	// First verify user exists
	user, err := store.Users().FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	fmt.Printf("Traffic Statistics for User: %s (ID: %d)\n", user.Email, user.ID)
	fmt.Printf("Period: Last %d days\n", days)
	fmt.Println("========================================")

	since := time.Now().AddDate(0, 0, -days).Unix()
	records, err := store.StatUsers().ListByUserSince(ctx, userID, since, days*2)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Println("No traffic records found for this period.")
		fmt.Println()
		fmt.Printf("Current Usage: ↑%s ↓%s\n",
			formatBytes(user.U),
			formatBytes(user.D))
		fmt.Printf("Transfer Limit: %s\n", formatBytes(user.TransferEnable))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DATE\tUPLOAD\tDOWNLOAD\tTOTAL")

	var totalUp, totalDown int64
	for _, r := range records {
		if r.RecordType != 1 {
			continue // Only show daily records
		}
		date := time.Unix(r.RecordAt, 0).Format("2006-01-02")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			date,
			formatBytes(r.Upload),
			formatBytes(r.Download),
			formatBytes(r.Upload+r.Download),
		)
		totalUp += r.Upload
		totalDown += r.Download
	}
	if err := w.Flush(); err != nil {
		slog.Warn("failed to flush stat output", "error", err)
	}

	fmt.Println("----------------------------------------")
	fmt.Printf("Period Total: ↑%s ↓%s = %s\n",
		formatBytes(totalUp),
		formatBytes(totalDown),
		formatBytes(totalUp+totalDown))
	fmt.Println()
	fmt.Printf("Current Usage: ↑%s ↓%s = %s\n",
		formatBytes(user.U),
		formatBytes(user.D),
		formatBytes(user.U+user.D))
	fmt.Printf("Transfer Limit: %s\n", formatBytes(user.TransferEnable))

	if user.TransferEnable > 0 {
		usage := float64(user.U+user.D) / float64(user.TransferEnable) * 100
		fmt.Printf("Usage: %.1f%%\n", usage)
	}

	return nil
}

func runStatTop(store *sqlite.Store, limit, days int) error {
	ctx := context.Background()

	startAt := time.Now().AddDate(0, 0, -days).Unix()
	endAt := time.Now().Unix()

	filter := repository.StatUserTopFilter{
		RecordType: 1, // daily
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      limit,
	}

	topUsers, err := store.StatUsers().TopByRange(ctx, filter)
	if err != nil {
		return err
	}

	if len(topUsers) == 0 {
		fmt.Println("No traffic data found for this period.")
		return nil
	}

	fmt.Printf("Top %d Users by Traffic (Last %d days)\n", limit, days)
	fmt.Println("========================================")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RANK\tUSER ID\tEMAIL\tUPLOAD\tDOWNLOAD\tTOTAL")

	for i, u := range topUsers {
		email := "Unknown"
		if user, err := store.Users().FindByID(ctx, u.UserID); err == nil {
			email = user.Email
		}
		fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\t%s\n",
			i+1,
			u.UserID,
			email,
			formatBytes(u.Upload),
			formatBytes(u.Download),
			formatBytes(u.Upload+u.Download),
		)
	}
	if err := w.Flush(); err != nil {
		slog.Warn("failed to flush stat output", "error", err)
	}

	return nil
}

func runStatTraffic(store *sqlite.Store, days int) error {
	ctx := context.Background()

	now := time.Now()
	startAt := now.AddDate(0, 0, -days).Unix()
	endAt := now.Unix()

	// User traffic
	userFilter := repository.StatUserSumFilter{
		RecordType: 1,
		StartAt:    startAt,
		EndAt:      endAt,
	}
	userTraffic, err := store.StatUsers().SumByRange(ctx, userFilter)
	if err != nil {
		return err
	}

	// Server traffic
	serverFilter := repository.StatServerSumFilter{
		RecordType: 1,
		StartAt:    startAt,
		EndAt:      endAt,
	}
	serverTraffic, err := store.StatServers().SumByRange(ctx, serverFilter)
	if err != nil {
		return err
	}

	fmt.Printf("Traffic Statistics (Last %d days)\n", days)
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("User Traffic (Billed):")
	fmt.Printf("  Upload:    %s\n", formatBytes(userTraffic.Upload))
	fmt.Printf("  Download:  %s\n", formatBytes(userTraffic.Download))
	fmt.Printf("  Total:     %s\n", formatBytes(userTraffic.Upload+userTraffic.Download))
	fmt.Println()
	fmt.Println("Node Traffic (NetIO):")
	fmt.Printf("  Upload:    %s\n", formatBytes(serverTraffic.Upload))
	fmt.Printf("  Download:  %s\n", formatBytes(serverTraffic.Download))
	fmt.Printf("  Total:     %s\n", formatBytes(serverTraffic.Upload+serverTraffic.Download))

	return nil
}
