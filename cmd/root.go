package cmd

import (
	"fmt"
	"os"

	"github.com/oxgrad/knot/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dryRun  bool
)

var rootCmd = &cobra.Command{
	Use:   "knot",
	Short: "A lightweight, configurable dotfiles manager",
	Long: `Knot manages your dotfiles via symlinks.
It reads a Knotfile and creates or removes symlinks
based on your package definitions.`,
	RunE: runTUI,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to Knotfile (default: $HOME/.dotfiles/Knotfile, override via KNOT_DIR)")
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
// If all is true, returns all package names from cfg.
// Otherwise validates and returns the provided args.
func resolvePackageArgs(args []string, all bool, cfg *config.Config) ([]string, error) {
	if all {
		names := make([]string, 0, len(cfg.Packages))
		for name := range cfg.Packages {
			names = append(names, name)
		}
		return names, nil
	}

	for _, name := range args {
		if _, ok := cfg.Packages[name]; !ok {
			return nil, fmt.Errorf("unknown package %q", name)
		}
	}
	return args, nil
}
