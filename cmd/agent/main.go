package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/creamcroissant/xboard/internal/agent/config"
	"github.com/creamcroissant/xboard/internal/agent/service"
	"github.com/creamcroissant/xboard/internal/support/logging"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"

	configFile string
	version    bool
)

func init() {
	flag.StringVar(&configFile, "config", "config.yml", "Path to configuration file")
	flag.BoolVar(&version, "version", false, "Show version")
	flag.Parse()
}

func main() {
	if version {
		fmt.Printf("XBoard Agent %s\n", Version)
		return
	}

	// Load Config first (needed for log settings)
	cfg, err := config.Load(configFile)
	if err != nil {
		slog.Error("Failed to load config", "path", configFile, "error", err)
		os.Exit(1)
	}

	// Setup Logger (stdout + optional daily-rotated file)
	logger := logging.New(logging.Options{
		Level:     slog.LevelInfo,
		Format:    "text",
		AddSource: false,
		LogDir:    cfg.Log.Dir,
		MaxDays:   cfg.Log.MaxDays,
	})
	slog.SetDefault(logger)
	if strings.TrimSpace(cfg.Update.CurrentVersion) == "" {
		cfg.Update.CurrentVersion = Version
	}

	// Initialize Agent
	agent, err := service.New(cfg)
	if err != nil {
		slog.Error("Failed to initialize agent", "error", err)
		os.Exit(1)
	}

	// Context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		select {
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down...", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	// Run
	agent.Run(ctx)
}
