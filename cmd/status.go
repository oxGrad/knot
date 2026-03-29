package cmd

import (
	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show symlink status for all packages",
	Long:  `Status displays the current state of all symlinks managed by knot.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig()
		if err != nil {
			return err
		}

		lnk := linker.New(false) // status is always read-only
		return lnk.Status(cfg)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
