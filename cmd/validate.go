package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/resolver"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the Knotfile for errors and warnings",
	Long: `Validate checks the Knotfile for structural correctness:
  - required fields (source, target) are present for every package
  - source directories exist on disk
  - condition.os values are known OS names
  - ignore patterns are valid glob expressions

Exit codes:
  0  valid, no issues
  1  one or more errors
  2  no errors, but warnings present`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, path, err := loadConfig()
		if err != nil {
			return err
		}

		home, _ := os.UserHomeDir()
		fmt.Printf("Validating Knotfile: %s\n\n", path)

		errs, warns := runValidation(cfg, home)

		for _, e := range errs {
			fmt.Printf("  ERROR %s\n", e)
		}
		for _, w := range warns {
			fmt.Printf("  WARN  %s\n", w)
		}
		fmt.Println()

		switch {
		case len(errs) > 0:
			fmt.Printf("Validation failed: %d error(s), %d warning(s)\n", len(errs), len(warns))
			return &ExitError{Code: 1}
		case len(warns) > 0:
			fmt.Printf("Validation passed with %d warning(s).\n", len(warns))
			return &ExitError{Code: 2}
		default:
			fmt.Printf("  OK: %d package(s) valid\n\nValidation passed.\n", len(cfg.Packages))
			return nil
		}
	},
}

// runValidation checks cfg for errors and warnings, returning them sorted.
func runValidation(cfg *config.Config, home string) (errs, warns []string) {
	if len(cfg.Packages) == 0 {
		warns = append(warns, "no packages defined")
		return
	}

	names := make([]string, 0, len(cfg.Packages))
	for n := range cfg.Packages {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		pkg := cfg.Packages[name]

		if pkg.Source == "" {
			errs = append(errs, fmt.Sprintf(`[%s]: "source" is required`, name))
		} else {
			expanded := resolver.ExpandPath(pkg.Source, home)
			info, statErr := os.Stat(expanded)
			if statErr != nil {
				errs = append(errs, fmt.Sprintf("[%s]: source directory %q does not exist", name, expanded))
			} else if !info.IsDir() {
				errs = append(errs, fmt.Sprintf("[%s]: source %q is not a directory", name, expanded))
			}
		}

		if pkg.Target == "" {
			errs = append(errs, fmt.Sprintf(`[%s]: "target" is required`, name))
		}

		if pkg.Condition != nil && pkg.Condition.OS != "" && !config.KnownOS[pkg.Condition.OS] {
			errs = append(errs, fmt.Sprintf(
				"[%s]: unknown condition.os value %q (must be one of: darwin, linux, windows, freebsd)",
				name, pkg.Condition.OS))
		}

		for _, pattern := range pkg.Ignore {
			if _, matchErr := resolver.ShouldIgnore("test", []string{pattern}); matchErr != nil {
				errs = append(errs, fmt.Sprintf("[%s]: invalid ignore pattern %q: %v", name, pattern, matchErr))
			}
		}
	}
	return
}

func init() {
	validateCmd.SilenceUsage = true
	rootCmd.AddCommand(validateCmd)
}
