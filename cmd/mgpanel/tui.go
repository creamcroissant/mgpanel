package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/creamcroissant/mgpanel/internal/bootstrap"
	"github.com/creamcroissant/mgpanel/internal/config"
	"github.com/creamcroissant/mgpanel/internal/migrations"
	"github.com/creamcroissant/mgpanel/internal/repository/sqlite"
	"github.com/creamcroissant/mgpanel/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive node monitor",
	Long:  "Launch an interactive terminal UI to monitor node status in real-time.",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := bootstrap.OpenSQLite(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := migrations.Up(db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	store := sqlite.NewStore(db)

	model := tui.NewModel(store)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}
