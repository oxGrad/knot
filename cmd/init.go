package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"

	"github.com/oxgrad/knot/internal/config"
	"github.com/spf13/cobra"
)

//go:embed examples/Knotfile
var exampleKnotfile []byte

var initCmd = &cobra.Command{
	Use:   "init [git-url]",
	Short: "Bootstrap a dotfiles directory",
	Long: `Initialize a new dotfiles setup.

Without arguments, creates a starter Knotfile at the default location
($HOME/.dotfiles/Knotfile, or $KNOT_DIR/Knotfile if KNOT_DIR is set).

With a git URL, clones the repository into the default dotfiles directory
and uses its Knotfile.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home dir: %w", err)
		}
		dir := config.DefaultDir(home)

		if len(args) == 1 {
			return cloneDotfiles(args[0], dir)
		}
		return scaffoldKnotfile(dir)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func scaffoldKnotfile(dir string) error {
	knotfilePath := fmt.Sprintf("%s/%s", dir, config.KnotfileName)

	if _, err := os.Stat(knotfilePath); err == nil {
		return fmt.Errorf("Knotfile already exists at %s\nUse --config to specify a different path, or edit it directly", knotfilePath)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(knotfilePath, exampleKnotfile, 0644); err != nil {
		return fmt.Errorf("writing Knotfile: %w", err)
	}

	fmt.Printf("Created %s\nEdit it to define your dotfile packages, then run 'knot tie --all'\n", knotfilePath)
	return nil
}

func cloneDotfiles(url, dir string) error {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("directory %s already exists and is not empty", dir)
	}

	fmt.Printf("Cloning %s into %s...\n", url, dir)
	c := exec.Command("git", "clone", url, dir)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	knotfilePath := fmt.Sprintf("%s/%s", dir, config.KnotfileName)
	if _, err := os.Stat(knotfilePath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: no Knotfile found at %s — check the repository structure\n", knotfilePath)
	} else {
		fmt.Printf("Cloned %s → %s\nRun 'knot tie --all' to apply your dotfiles\n", url, dir)
	}
	return nil
}
