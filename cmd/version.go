package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time by GoReleaser via ldflags.
// It defaults to "dev" for local builds.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the knot version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
