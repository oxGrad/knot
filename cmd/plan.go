package cmd

import (
	"fmt"

	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

var (
	planAll bool
	planTag string
)

var planCmd = &cobra.Command{
	Use:   "plan [package...]",
	Short: "Dry-run: show what tie would do without making changes",
	Long: `Plan shows exactly what symlinks knot would create or remove,
without actually modifying the filesystem.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasArgs := len(args) > 0
		hasTag := planTag != ""

		if hasTag && planAll {
			return fmt.Errorf("cannot use --tag with --all")
		}
		if hasTag && hasArgs {
			return fmt.Errorf("cannot use --tag with package names")
		}
		if !planAll && !hasTag && !hasArgs {
			return fmt.Errorf("specify at least one package or use --all")
		}

		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		var names []string
		if hasTag {
			names, err = resolveTagArg(planTag, cfg)
		} else {
			names, err = resolvePackageArgs(args, planAll, cfg)
		}
		if err != nil {
			return err
		}

		lnk := linker.New(true) // plan is always dry-run
		actions, err := lnk.Plan(cfg, names)
		if err != nil {
			return err
		}
		lnk.PrintPlan(actions)
		return nil
	},
}

func init() {
	planCmd.Flags().BoolVar(&planAll, "all", false, "plan all packages")
	planCmd.Flags().StringVar(&planTag, "tag", "", "plan all packages with this tag")
	rootCmd.AddCommand(planCmd)
}
