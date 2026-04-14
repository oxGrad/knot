package cmd

import (
	"fmt"

	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

var (
	tieAll bool
	tieTag string
)

var tieCmd = &cobra.Command{
	Use:   "tie [package...]",
	Short: "Create symlinks for one or more packages",
	Long: `Tie creates symlinks for the specified packages.
Use --all to tie all packages, or --tag <name> to tie all packages in a tag.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasArgs := len(args) > 0
		hasTag := tieTag != ""

		if hasTag && tieAll {
			return fmt.Errorf("cannot use --tag with --all")
		}
		if hasTag && hasArgs {
			return fmt.Errorf("cannot use --tag with package names")
		}
		if !tieAll && !hasTag && !hasArgs {
			return fmt.Errorf("specify at least one package, --all, or --tag <name>")
		}

		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		var names []string
		if hasTag {
			names, err = resolveTagArg(tieTag, cfg)
		} else {
			names, err = resolvePackageArgs(args, tieAll, cfg)
		}
		if err != nil {
			return err
		}

		lnk := linker.New(dryRun)
		actions, err := lnk.Plan(cfg, names)
		if err != nil {
			return err
		}
		return lnk.Apply(actions)
	},
}

func init() {
	tieCmd.Flags().BoolVar(&tieAll, "all", false, "tie all packages")
	tieCmd.Flags().StringVar(&tieTag, "tag", "", "tie all packages with this tag")
	rootCmd.AddCommand(tieCmd)
}
