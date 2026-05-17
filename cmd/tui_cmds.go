package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)

var versionRe = regexp.MustCompile(`v?\d+\.\d+(?:\.\d+)*[a-zA-Z]?(?:[-+][a-zA-Z0-9.]+)*`)

func extractVersion(output string) string {
	return versionRe.FindString(output)
}

func dotfilesDir(cfgPath string) string {
	return filepath.Dir(cfgPath)
}

func headerTickCmd() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(time.Time) tea.Msg {
		return headerTickMsg{}
	})
}

func fetchGitInfoCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		shaOut, err := exec.Command("git", "-C", dir, "log", "-1", "--pretty=format:%h").Output()
		if err != nil {
			return gitInfoMsg{err: err}
		}
		msgOut, _ := exec.Command("git", "-C", dir, "log", "-1", "--pretty=format:%s").Output()
		branchOut, _ := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
		return gitInfoMsg{
			sha:    strings.TrimSpace(string(shaOut)),
			msg:    strings.TrimSpace(string(msgOut)),
			branch: strings.TrimSpace(string(branchOut)),
		}
	}
}

func fetchBranchesCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("git", "-C", dir, "branch", "--format=%(refname:short)").Output()
		if err != nil {
			return branchListMsg{err: err}
		}
		var branches []string
		for _, b := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			b = strings.TrimSpace(b)
			if b != "" {
				branches = append(branches, b)
			}
		}
		return branchListMsg{branches: branches}
	}
}

func checkoutBranchCmd(dir, branch string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("git", "-C", dir, "checkout", branch).CombinedOutput()
		return checkoutDoneMsg{output: string(out), err: err}
	}
}

func gitPullCmd(cfgPath string) tea.Cmd {
	return func() tea.Msg {
		dir := dotfilesDir(cfgPath)
		cmd := exec.Command("git", "pull")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return gitPullResultMsg{output: string(out), err: err}
	}
}

func reloadCmd(cfgPath string, lnk *linker.Linker) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return reloadMsg{err: err}
		}
		rows, err := buildRows(cfg, lnk)
		return reloadMsg{rows: rows, cfg: cfg, err: err}
	}
}

func applyCmd(cfg *config.Config, lnk *linker.Linker, rows []pkgRow, toggles map[string]bool) tea.Cmd {
	return func() tea.Msg {
		var log []string
		var errs []error

		for _, row := range rows {
			wantTied := toggles[row.name]
			currentlyTied := row.status == statusTied || row.status == statusPartial
			if wantTied == currentlyTied {
				continue
			}

			var actions []linker.LinkAction
			var err error
			if wantTied {
				actions, err = lnk.Plan(cfg, []string{row.name})
			} else {
				actions, err = lnk.PlanUntie(cfg, []string{row.name})
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}

			var buf bytes.Buffer
			lnk.Writer = &buf
			if applyErr := lnk.Apply(actions); applyErr != nil {
				errs = append(errs, applyErr)
			}
			lnk.Writer = os.Stdout

			for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
				if line != "" {
					log = append(log, line)
				}
			}
		}

		return applyDoneMsg{log: log, err: errors.Join(errs...)}
	}
}

// checkVersionCmd checks if binName is in PATH and retrieves its version string.
// It tries --version first, then falls back to the version subcommand (e.g. k9s).
func checkVersionCmd(pkgName, binName string) tea.Cmd {
	return func() tea.Msg {
		path, err := exec.LookPath(binName)
		if err != nil {
			return versionCheckMsg{pkgName: pkgName, found: false}
		}
		for _, args := range [][]string{{"--version"}, {"-V"}, {"version"}} {
			out, _ := exec.Command(path, args...).CombinedOutput()
			if v := extractVersion(string(out)); v != "" {
				return versionCheckMsg{pkgName: pkgName, found: true, version: v}
			}
		}
		return versionCheckMsg{pkgName: pkgName, found: true, version: ""}
	}
}

// installCmd wraps a pre-built exec.Cmd in tea.ExecProcess, returning installDoneMsg when done.
func installCmd(pkgName string, c *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return installDoneMsg{pkgName: pkgName, err: err}
	})
}

// buildInstallCommand constructs the OS command for the given manager and install config.
// Returns nil if the relevant field is empty.
func buildInstallCommand(kind pkgManagerKind, install *config.Install) *exec.Cmd {
	switch kind {
	case pkgMgrBrew:
		if install.Brew == "" {
			return nil
		}
		return exec.Command("brew", "install", install.Brew)
	case pkgMgrApt:
		if install.Apt == "" {
			return nil
		}
		return exec.Command("sudo", "apt-get", "install", "-y", install.Apt)
	case pkgMgrDnf:
		if install.Dnf == "" {
			return nil
		}
		return exec.Command("sudo", "dnf", "install", "-y", install.Dnf)
	case pkgMgrScript:
		if install.Script == "" {
			return nil
		}
		return exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s | bash", install.Script))
	}
	return nil
}

// detectAvailableManagers returns the configured managers and a map indicating PATH availability.
func detectAvailableManagers(install *config.Install) ([]pkgManagerKind, map[pkgManagerKind]bool) {
	var mgrs []pkgManagerKind
	avail := make(map[pkgManagerKind]bool)
	check := func(kind pkgManagerKind, field string) {
		if field == "" {
			return
		}
		mgrs = append(mgrs, kind)
		_, err := exec.LookPath(kind.binary())
		avail[kind] = (err == nil)
	}
	check(pkgMgrBrew, install.Brew)
	check(pkgMgrApt, install.Apt)
	check(pkgMgrDnf, install.Dnf)
	check(pkgMgrScript, install.Script)
	return mgrs, avail
}

// renderInstallCommandPreview returns the command string shown in the install selection UI.
func renderInstallCommandPreview(kind pkgManagerKind, install *config.Install) string {
	switch kind {
	case pkgMgrBrew:
		return "brew install " + install.Brew
	case pkgMgrApt:
		return "sudo apt-get install -y " + install.Apt
	case pkgMgrDnf:
		return "sudo dnf install -y " + install.Dnf
	case pkgMgrScript:
		return "curl -fsSL " + install.Script + " | bash"
	}
	return ""
}

func editorCmd(cfgPath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	dir := dotfilesDir(cfgPath)
	c := exec.Command(editor, dir)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}
