package cmd

import (
	"fmt"

	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

var (
	untieAll bool
	untieTag string
)

var untieCmd = &cobra.Command{
	Use:   "untie [package...]",
	Short: "Remove symlinks for one or more packages",
	Long:  `Untie removes symlinks previously created by knot tie.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasArgs := len(args) > 0
		hasTag := untieTag != ""

		if hasTag && untieAll {
			return fmt.Errorf("cannot use --tag with --all")
		}
		if hasTag && hasArgs {
			return fmt.Errorf("cannot use --tag with package names")
		}
		if !untieAll && !hasTag && !hasArgs {
			return fmt.Errorf("specify at least one package, --all, or --tag <name>")
		}

		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		var names []string
		if hasTag {
			names, err = resolveTagArg(untieTag, cfg)
		} else {
			names, err = resolvePackageArgs(args, untieAll, cfg)
		}
		if err != nil {
			return err
		}

		lnk := linker.New(dryRun)
		actions, err := lnk.PlanUntie(cfg, names)
		if err != nil {
			return err
		}
		return lnk.Apply(actions)
	},
}

func init() {
	untieCmd.Flags().BoolVar(&untieAll, "all", false, "untie all packages")
	untieCmd.Flags().StringVar(&untieTag, "tag", "", "untie all packages with this tag")
	rootCmd.AddCommand(untieCmd)
}
