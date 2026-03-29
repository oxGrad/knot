package cmd

import (
	"fmt"

	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

var tieAll bool

var tieCmd = &cobra.Command{
	Use:   "tie [package...]",
	Short: "Create symlinks for one or more packages",
	Long: `Tie creates symlinks for the specified packages.
Use --all to tie all packages defined in knot.yml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !tieAll && len(args) == 0 {
			return fmt.Errorf("specify at least one package or use --all")
		}

		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		names, err := resolvePackageArgs(args, tieAll, cfg)
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
	rootCmd.AddCommand(tieCmd)
}
