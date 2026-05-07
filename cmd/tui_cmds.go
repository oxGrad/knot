package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)

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
