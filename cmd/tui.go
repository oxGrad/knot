package cmd

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
	"github.com/oxgrad/knot/internal/setup"
	"github.com/spf13/cobra"
)


// ── entry point ───────────────────────────────────────────────────────────────

func brandHeaderFn(width int) string {
	return model{width: width}.renderBrandHeader()
}

func runTUI(cmd *cobra.Command, args []string) error {
	cfg, cfgPath, err := loadConfig()
	if err != nil {
		if cfgFile == "" {
			home, _ := os.UserHomeDir()
			dir := config.DefaultDir(home)
			knotfile := config.DefaultKnotfilePath(home)
			_, dirErr := os.Stat(dir)
			_, knotfileErr := os.Stat(knotfile)
			switch {
			case os.IsNotExist(dirErr):
				if wizErr := setup.Run(dir, setup.ModeInit, brandHeaderFn, exampleKnotfile); wizErr != nil {
					return wizErr
				}
				cfg, cfgPath, err = loadConfig()
			case os.IsNotExist(knotfileErr):
				wizErr := setup.Run(dir, setup.ModeKnotfile, brandHeaderFn, exampleKnotfile)
				if errors.Is(wizErr, setup.ErrDeclined) {
					fmt.Fprintf(os.Stderr, "No Knotfile in %s. Run 'knot init' to create one.\n", dir)
					return nil
				}
				if wizErr != nil {
					return wizErr
				}
				cfg, cfgPath, err = loadConfig()
			}
		}
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	}

	lnk := linker.New(false)
	rows, err := buildRows(cfg, lnk)
	if err != nil {
		return fmt.Errorf("computing status: %w", err)
	}

	m := model{
		cfg:     cfg,
		cfgPath: cfgPath,
		lnk:     lnk,
		rows:    rows,
		toggles: seedToggles(rows),
		tagRows: buildTagRows(cfg, rows),
		phase:   phaseList,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// ── utils ─────────────────────────────────────────────────────────────────────

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
