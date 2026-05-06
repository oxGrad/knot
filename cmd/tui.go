package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)


// ── entry point ───────────────────────────────────────────────────────────────

func runTUI(cmd *cobra.Command, args []string) error {
	cfg, cfgPath, err := loadConfig()
	if err != nil {
		if cfgFile == "" {
			home, _ := os.UserHomeDir()
			dir := config.DefaultDir(home)
			knotfile := config.DefaultKnotfilePath(home)
			_, dirErr := os.Stat(dir)
			_, knotfileErr := os.Stat(knotfile)
			switch {
			case os.IsNotExist(dirErr):
				if wizErr := runSetupWizard(dir, setupModeInit); wizErr != nil {
					return wizErr
				}
				cfg, cfgPath, err = loadConfig()
			case os.IsNotExist(knotfileErr):
				wizErr := runSetupWizard(dir, setupModeKnotfile)
				if wizErr == errSetupDeclined {
					fmt.Fprintf(os.Stderr, "No Knotfile in %s. Run 'knot init' to create one.\n", dir)
					return nil
				}
				if wizErr != nil {
					return wizErr
				}
				cfg, cfgPath, err = loadConfig()
			}
		}
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	}

	lnk := linker.New(false)
	rows, err := buildRows(cfg, lnk)
	if err != nil {
		return fmt.Errorf("computing status: %w", err)
	}

	m := model{
		cfg:     cfg,
		cfgPath: cfgPath,
		lnk:     lnk,
		rows:    rows,
		toggles: seedToggles(rows),
		tagRows: buildTagRows(cfg, rows),
		phase:   phaseList,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// ── setup wizard ─────────────────────────────────────────────────────────────

type setupMode int

const (
	setupModeInit     setupMode = iota // dotfiles dir missing: show init/clone menu
	setupModeKnotfile                  // dir present, Knotfile missing: confirm only
)

type setupPhase int

const (
	setupPhaseMenu            setupPhase = iota // choose: initialize or clone from git
	setupPhaseGitProvider                       // choose: GitHub or GitLab
	setupPhaseGitProtocol                       // choose: HTTPS or SSH
	setupPhaseGitUsername                       // text input for username
	setupPhaseGitRepo                           // text input for repo name
	setupPhaseCloning                           // running git clone
	setupPhaseConfirmKnotfile                   // dir exists, no Knotfile: y/n prompt
	setupPhaseDone                              // success, about to exit
)

type setupModel struct {
	dir         string
	mode        setupMode
	phase       setupPhase
	cursor      int    // menu cursor
	inputBuf    string // accumulates typed characters
	gitProvider string // "github" or "gitlab"
	gitProtocol string // "https" or "ssh"
	gitUsername string
	gitRepo     string
	cloneURL    string // fully constructed URL for cloning
	err         error  // last error to display
	declined    bool   // user chose not to create Knotfile
	width       int
}

func buildGitURL(provider, protocol, username, repo string) string {
	if repo == "" {
		repo = ".dotfiles"
	}
	switch {
	case provider == "github" && protocol == "https":
		return fmt.Sprintf("https://github.com/%s/%s", username, repo)
	case provider == "github" && protocol == "ssh":
		return fmt.Sprintf("git@github.com:%s/%s.git", username, repo)
	case provider == "gitlab" && protocol == "https":
		return fmt.Sprintf("https://gitlab.com/%s/%s", username, repo)
	case provider == "gitlab" && protocol == "ssh":
		return fmt.Sprintf("git@gitlab.com:%s/%s.git", username, repo)
	}
	return ""
}

type (
	cloneDoneMsg     struct{ err error }
	knotfileReadyMsg struct{ err error }
)

func cloneRepoCmd(url, dir string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("git", "clone", url, dir)
		if err := c.Run(); err != nil {
			return cloneDoneMsg{err: fmt.Errorf("git clone failed: %w", err)}
		}
		return cloneDoneMsg{}
	}
}

func writeKnotfileCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		knotfilePath := filepath.Join(dir, config.KnotfileName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return knotfileReadyMsg{err: fmt.Errorf("creating directory: %w", err)}
		}
		if err := os.WriteFile(knotfilePath, exampleKnotfile, 0o644); err != nil {
			return knotfileReadyMsg{err: fmt.Errorf("writing Knotfile: %w", err)}
		}
		return knotfileReadyMsg{}
	}
}

func (m setupModel) Init() tea.Cmd {
	if m.mode == setupModeKnotfile {
		return func() tea.Msg { return nil }
	}
	return nil
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case cloneDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = setupPhaseGitRepo
			return m, nil
		}
		// After clone: if no Knotfile, create from template automatically
		knotfilePath := filepath.Join(m.dir, config.KnotfileName)
		if _, err := os.Stat(knotfilePath); os.IsNotExist(err) {
			return m, writeKnotfileCmd(m.dir)
		}
		m.phase = setupPhaseDone
		return m, tea.Quit

	case knotfileReadyMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.phase = setupPhaseDone
		return m, tea.Quit

	case tea.KeyMsg:
		switch m.phase {
		case setupPhaseMenu:
			return m.updateMenu(msg)
		case setupPhaseGitProvider:
			return m.updateGitProvider(msg)
		case setupPhaseGitProtocol:
			return m.updateGitProtocol(msg)
		case setupPhaseGitUsername:
			return m.updateGitInput(msg, setupPhaseGitProtocol, setupPhaseGitRepo)
		case setupPhaseGitRepo:
			return m.updateGitInput(msg, setupPhaseGitUsername, setupPhaseGitRepo)
		case setupPhaseConfirmKnotfile:
			return m.updateConfirmKnotfile(msg)
		}
	}
	return m, nil
}

func (m setupModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		m.err = nil
		if m.cursor == 0 {
			return m, writeKnotfileCmd(m.dir)
		}
		// Clone from git — start guided flow
		m.phase = setupPhaseGitProvider
		m.cursor = 0
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m setupModel) updateGitProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor == 0 {
			m.gitProvider = "github"
		} else {
			m.gitProvider = "gitlab"
		}
		m.cursor = 0
		m.phase = setupPhaseGitProtocol
	case "esc":
		m.phase = setupPhaseMenu
		m.cursor = 1 // restore "Clone from git" selection
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m setupModel) updateGitProtocol(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor == 0 {
			m.gitProtocol = "https"
		} else {
			m.gitProtocol = "ssh"
		}
		m.cursor = 0
		m.inputBuf = ""
		m.phase = setupPhaseGitUsername
	case "esc":
		m.phase = setupPhaseGitProvider
		m.cursor = 0
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// updateGitInput handles text entry for both the username and repo phases.
// backPhase is where esc goes; nextPhase is where enter goes (or clones if repo phase).
func (m setupModel) updateGitInput(msg tea.KeyMsg, backPhase, nextPhase setupPhase) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		m.inputBuf += msg.String()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.inputBuf) > 0 {
			runes := []rune(m.inputBuf)
			m.inputBuf = string(runes[:len(runes)-1])
		}
	case tea.KeyEnter:
		if m.phase == setupPhaseGitUsername {
			username := strings.TrimSpace(m.inputBuf)
			if username == "" {
				return m, nil
			}
			m.gitUsername = username
			m.inputBuf = ""
			m.phase = setupPhaseGitRepo
		} else {
			// repo phase — build URL and clone
			repo := strings.TrimSpace(m.inputBuf)
			if repo == "" {
				repo = ".dotfiles"
			}
			m.gitRepo = repo
			m.cloneURL = buildGitURL(m.gitProvider, m.gitProtocol, m.gitUsername, repo)
			m.err = nil
			m.phase = setupPhaseCloning
			return m, cloneRepoCmd(m.cloneURL, m.dir)
		}
	case tea.KeyEsc:
		m.inputBuf = ""
		m.err = nil
		m.phase = backPhase
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	_ = nextPhase
	return m, nil
}

func (m setupModel) updateConfirmKnotfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		return m, writeKnotfileCmd(m.dir)
	case "n", "esc", "q", "ctrl+c":
		m.declined = true
		return m, tea.Quit
	}
	return m, nil
}

func (m setupModel) View() string {
	var b strings.Builder

	b.WriteString(styleBold.Render("knot setup") + "\n\n")

	switch m.phase {
	case setupPhaseMenu:
		b.WriteString("No dotfiles directory found at " + styleCyan.Render(m.dir) + ".\n\n")
		opts := []string{"Initialize new dotfiles folder", "Clone existing dotfiles from git"}
		for i, opt := range opts {
			if i == m.cursor {
				b.WriteString(styleCursor.Render("> ") + styleBold.Render(opt) + "\n")
			} else {
				b.WriteString("  " + opt + "\n")
			}
		}
		b.WriteString("\n" + styleDim.Render("↑/↓ to move · enter to select · q to quit"))

	case setupPhaseGitProvider:
		b.WriteString("Select a git provider:\n\n")
		providers := []string{"GitHub", "GitLab"}
		for i, p := range providers {
			if i == m.cursor {
				b.WriteString(styleCursor.Render("> ") + styleBold.Render(p) + "\n")
			} else {
				b.WriteString("  " + p + "\n")
			}
		}
		b.WriteString("\n" + styleDim.Render("↑/↓ to move · enter to select · esc to go back"))

	case setupPhaseGitProtocol:
		host := "github.com"
		if m.gitProvider == "gitlab" {
			host = "gitlab.com"
		}
		b.WriteString("Select a protocol for " + styleCyan.Render(host) + ":\n\n")
		protocols := []string{"HTTPS", "SSH"}
		for i, p := range protocols {
			if i == m.cursor {
				b.WriteString(styleCursor.Render("> ") + styleBold.Render(p) + "\n")
			} else {
				b.WriteString("  " + p + "\n")
			}
		}
		b.WriteString("\n" + styleDim.Render("↑/↓ to move · enter to select · esc to go back"))
		b.WriteString("\n" + styleDim.Render("Note: SSH requires your public key to be added to your "+host+" account."))

	case setupPhaseGitUsername:
		b.WriteString("Enter your " + styleCyan.Render(m.gitProvider) + " username:\n\n")
		b.WriteString("  " + styleCyan.Render(m.inputBuf) + "█\n")
		b.WriteString("\n" + styleDim.Render("enter to confirm · esc to go back · ctrl+c to quit"))

	case setupPhaseGitRepo:
		b.WriteString("Enter the repository name:\n\n")
		b.WriteString("  " + styleCyan.Render(m.inputBuf) + "█\n")
		b.WriteString("\n" + styleDim.Render("enter to confirm · esc to go back · ctrl+c to quit"))
		b.WriteString("\n" + styleDim.Render("Leave empty to use the default: ") + styleCyan.Render(".dotfiles"))
		if m.err != nil {
			b.WriteString("\n\n" + styleRed.Render(m.err.Error()))
		}

	case setupPhaseCloning:
		b.WriteString("Cloning " + styleCyan.Render(m.cloneURL) + "\n")
		b.WriteString("into    " + styleCyan.Render(m.dir) + "\n\n")
		b.WriteString(styleDim.Render("Please wait…"))

	case setupPhaseConfirmKnotfile:
		b.WriteString("No Knotfile found in " + styleCyan.Render(m.dir) + ".\n\n")
		b.WriteString("Create one from template? " + styleBold.Render("[y/n]") + "\n")
		if m.err != nil {
			b.WriteString("\n" + styleRed.Render(m.err.Error()))
		}

	case setupPhaseDone:
		b.WriteString(styleGreen.Render("Setup complete.") + " Starting knot…")
	}

	return b.String()
}

// errSetupDeclined is returned by runSetupWizard when the user explicitly
// chose not to create a Knotfile. It is not a real error — callers should
// exit cleanly and print a helpful hint instead.
var errSetupDeclined = fmt.Errorf("setup declined")

func runSetupWizard(dir string, mode setupMode) error {
	phase := setupPhaseMenu
	if mode == setupModeKnotfile {
		phase = setupPhaseConfirmKnotfile
	}
	m := setupModel{dir: dir, mode: mode, phase: phase}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	sm, ok := final.(setupModel)
	if !ok {
		return nil
	}
	if sm.err != nil {
		return sm.err
	}
	if sm.declined {
		return errSetupDeclined
	}
	return nil
}

// ── utils ─────────────────────────────────────────────────────────────────────

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
