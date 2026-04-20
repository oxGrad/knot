package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/oxgrad/knot/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dryRun  bool
)

// ExitError signals a specific OS exit code to Execute without triggering
// Cobra's error printer — the command is responsible for its own output.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return "" }

var rootCmd = &cobra.Command{
	Use:   "knot",
	Short: "A lightweight, configurable dotfiles manager",
	Long: `Knot manages your dotfiles via symlinks.
It reads a Knotfile and creates or removes symlinks
based on your package definitions.`,
}

// Execute runs the root command.
func Execute() {
	rootCmd.SilenceErrors = true // we handle printing to avoid Cobra's double-print
	if err := rootCmd.Execute(); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		`path to Knotfile (default: $HOME/.dotfiles/Knotfile, override via KNOT_DIR)`)
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print actions without executing them")
}

// loadConfig finds and parses the Knotfile.
func loadConfig() (*config.Config, string, error) {
	if cfgFile != "" {
		cfg, err := config.Load(cfgFile)
		return cfg, cfgFile, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("getting home dir: %w", err)
	}

	path := config.DefaultKnotfilePath(home)
	cfg, err := config.Load(path)
	return cfg, path, err
}

// resolvePackageArgs returns the list of packages to operate on.
// If all is true, returns all package names from cfg sorted alphabetically.
// Otherwise returns args as-is; the linker validates individual names.
func resolvePackageArgs(args []string, all bool, cfg *config.Config) ([]string, error) {
	if all {
		names := make([]string, 0, len(cfg.Packages))
		for name := range cfg.Packages {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, nil
	}
	return args, nil
}
