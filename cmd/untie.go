package cmd

import (
	"fmt"

	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

var untieCmd = &cobra.Command{
	Use:   "untie [package...]",
	Short: "Remove symlinks for one or more packages",
	Long:  `Untie removes symlinks previously created by knot tie.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify at least one package")
		}

		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		names, err := resolvePackageArgs(args, false, cfg)
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
	rootCmd.AddCommand(untieCmd)
}
