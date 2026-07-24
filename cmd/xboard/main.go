package main

import (
	"context"
	"fmt"
	"os"

	"github.com/creamcroissant/xboard/internal/config"
	"github.com/spf13/cobra"
)

// Build info - injected via ldflags
var (
	Version    = "dev"
	Commit     = "unknown"
	BuildTime  = "unknown"
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "xboard",
	Short: "XBoard Panel and Node",
	Long:  `XBoard is a panel and node server for managing proxies.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := prepareConfigForCommand(cmd); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return nil
	},
}

// configCtxKey is used to cache the loaded config in the command context.
type configCtxKey struct{}

func prepareConfigForCommand(cmd *cobra.Command) error {
	if cmd != nil && cmd.Name() == "serve" {
		if _, err := config.EnsureDefaultConfig(config.EnsureDefaultConfigOptions{ConfigPath: configPath}); err != nil {
			return fmt.Errorf("ensure default config: %w", err)
		}
	}
	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
	if err != nil {
		return err
	}
	// Cache config in command context for downstream use.
	parentCtx := cmd.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx := context.WithValue(parentCtx, configCtxKey{}, cfg)
	cmd.SetContext(ctx)
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to configuration file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
