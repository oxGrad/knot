package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oxgrad/knot/internal/config"
)

// ErrDeclined is returned when the user explicitly chose not to create a Knotfile.
var ErrDeclined = errors.New("setup declined")

// Mode controls which wizard flow to run.
type Mode int

const (
	ModeInit     Mode = iota // dotfiles dir missing: show init/clone menu
	ModeKnotfile             // dir present, Knotfile missing: confirm only
)

// Run runs the setup wizard TUI.
func Run(dir string, mode Mode, headerFn func(width int) string, knotfileTemplate []byte) error {
	ph := phaseMenu
	if mode == ModeKnotfile {
		ph = phaseConfirmKnotfile
	}
	m := model{
		dir:              dir,
		mode:             mode,
		phase:            ph,
		headerFn:         headerFn,
		knotfileTemplate: knotfileTemplate,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	sm, ok := final.(model)
	if !ok {
		return nil
	}
	if sm.err != nil {
		return sm.err
	}
	if sm.declined {
		return ErrDeclined
	}
	return nil
}

// ── local styles ──────────────────────────────────────────────────────────────

var (
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Faint(true)
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// ── internal types ────────────────────────────────────────────────────────────

type phase int

const (
	phaseMenu            phase = iota
	phaseGitProvider
	phaseGitProtocol
	phaseGitUsername
	phaseGitRepo
	phaseCloning
	phaseConfirmKnotfile
	phaseDone
)

type model struct {
	dir              string
	mode             Mode
	phase            phase
	cursor           int
	inputBuf         string
	gitProvider      string
	gitProtocol      string
	gitUsername      string
	gitRepo          string
	cloneURL         string
	err              error
	declined         bool
	width            int
	headerFn         func(width int) string
	knotfileTemplate []byte
}

type cloneDoneMsg struct{ err error }
type knotfileReadyMsg struct{ err error }

// ── tea.Cmds ──────────────────────────────────────────────────────────────────

func (m model) cloneRepoCmd(url, dir string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("git", "clone", url, dir)
		if err := c.Run(); err != nil {
			return cloneDoneMsg{err: fmt.Errorf("git clone failed: %w", err)}
		}
		return cloneDoneMsg{}
	}
}

func (m model) writeKnotfileCmd(dir string) tea.Cmd {
	tmpl := m.knotfileTemplate
	return func() tea.Msg {
		knotfilePath := filepath.Join(dir, config.KnotfileName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return knotfileReadyMsg{err: fmt.Errorf("creating directory: %w", err)}
		}
		if err := os.WriteFile(knotfilePath, tmpl, 0o644); err != nil {
			return knotfileReadyMsg{err: fmt.Errorf("writing Knotfile: %w", err)}
		}
		return knotfileReadyMsg{}
	}
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

// ── bubbletea interface ───────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	if m.mode == ModeKnotfile {
		return func() tea.Msg { return nil }
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case cloneDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.phase = phaseGitRepo
			return m, nil
		}
		knotfilePath := filepath.Join(m.dir, config.KnotfileName)
		if _, err := os.Stat(knotfilePath); os.IsNotExist(err) {
			return m, m.writeKnotfileCmd(m.dir)
		}
		m.phase = phaseDone
		return m, tea.Quit

	case knotfileReadyMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.phase = phaseDone
		return m, tea.Quit

	case tea.KeyMsg:
		switch m.phase {
		case phaseMenu:
			return m.updateMenu(msg)
		case phaseGitProvider:
			return m.updateGitProvider(msg)
		case phaseGitProtocol:
			return m.updateGitProtocol(msg)
		case phaseGitUsername:
			return m.updateGitInput(msg, phaseGitProtocol, phaseGitRepo)
		case phaseGitRepo:
			return m.updateGitInput(msg, phaseGitUsername, phaseGitRepo)
		case phaseConfirmKnotfile:
			return m.updateConfirmKnotfile(msg)
		}
	}
	return m, nil
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			return m, m.writeKnotfileCmd(m.dir)
		}
		m.phase = phaseGitProvider
		m.cursor = 0
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateGitProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.phase = phaseGitProtocol
	case "esc":
		m.phase = phaseMenu
		m.cursor = 1
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateGitProtocol(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.phase = phaseGitUsername
	case "esc":
		m.phase = phaseGitProvider
		m.cursor = 0
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateGitInput(msg tea.KeyMsg, backPhase, nextPhase phase) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		m.inputBuf += msg.String()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.inputBuf) > 0 {
			runes := []rune(m.inputBuf)
			m.inputBuf = string(runes[:len(runes)-1])
		}
	case tea.KeyEnter:
		if m.phase == phaseGitUsername {
			username := strings.TrimSpace(m.inputBuf)
			if username == "" {
				return m, nil
			}
			m.gitUsername = username
			m.inputBuf = ""
			m.phase = phaseGitRepo
		} else {
			repo := strings.TrimSpace(m.inputBuf)
			if repo == "" {
				repo = ".dotfiles"
			}
			m.gitRepo = repo
			m.cloneURL = buildGitURL(m.gitProvider, m.gitProtocol, m.gitUsername, repo)
			m.err = nil
			m.phase = phaseCloning
			return m, m.cloneRepoCmd(m.cloneURL, m.dir)
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

func (m model) updateConfirmKnotfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		return m, m.writeKnotfileCmd(m.dir)
	case "n", "esc", "q", "ctrl+c":
		m.declined = true
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	if m.headerFn != nil {
		b.WriteString(m.headerFn(m.width))
	}

	b.WriteString(boldStyle.Render("knot setup") + "\n\n")

	switch m.phase {
	case phaseMenu:
		b.WriteString("No dotfiles directory found at " + cyanStyle.Render(m.dir) + ".\n\n")
		opts := []string{"Initialize new dotfiles folder", "Clone existing dotfiles from git"}
		for i, opt := range opts {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> ") + boldStyle.Render(opt) + "\n")
			} else {
				b.WriteString("  " + opt + "\n")
			}
		}
		b.WriteString("\n" + dimStyle.Render("↑/↓ to move · enter to select · q to quit"))

	case phaseGitProvider:
		b.WriteString("Select a git provider:\n\n")
		providers := []string{"GitHub", "GitLab"}
		for i, p := range providers {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> ") + boldStyle.Render(p) + "\n")
			} else {
				b.WriteString("  " + p + "\n")
			}
		}
		b.WriteString("\n" + dimStyle.Render("↑/↓ to move · enter to select · esc to go back"))

	case phaseGitProtocol:
		host := "github.com"
		if m.gitProvider == "gitlab" {
			host = "gitlab.com"
		}
		b.WriteString("Select a protocol for " + cyanStyle.Render(host) + ":\n\n")
		protocols := []string{"HTTPS", "SSH"}
		for i, p := range protocols {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> ") + boldStyle.Render(p) + "\n")
			} else {
				b.WriteString("  " + p + "\n")
			}
		}
		b.WriteString("\n" + dimStyle.Render("↑/↓ to move · enter to select · esc to go back"))
		b.WriteString("\n" + dimStyle.Render("Note: SSH requires your public key to be added to your "+host+" account."))

	case phaseGitUsername:
		b.WriteString("Enter your " + cyanStyle.Render(m.gitProvider) + " username:\n\n")
		b.WriteString("  " + cyanStyle.Render(m.inputBuf) + "█\n")
		b.WriteString("\n" + dimStyle.Render("enter to confirm · esc to go back · ctrl+c to quit"))

	case phaseGitRepo:
		b.WriteString("Enter the repository name:\n\n")
		b.WriteString("  " + cyanStyle.Render(m.inputBuf) + "█\n")
		b.WriteString("\n" + dimStyle.Render("enter to confirm · esc to go back · ctrl+c to quit"))
		b.WriteString("\n" + dimStyle.Render("Leave empty to use the default: ") + cyanStyle.Render(".dotfiles"))
		if m.err != nil {
			b.WriteString("\n\n" + redStyle.Render(m.err.Error()))
		}

	case phaseCloning:
		b.WriteString("Cloning " + cyanStyle.Render(m.cloneURL) + "\n")
		b.WriteString("into    " + cyanStyle.Render(m.dir) + "\n\n")
		b.WriteString(dimStyle.Render("Please wait…"))

	case phaseConfirmKnotfile:
		b.WriteString("No Knotfile found in " + cyanStyle.Render(m.dir) + ".\n\n")
		b.WriteString("Create one from template? " + boldStyle.Render("[y/n]") + "\n")
		if m.err != nil {
			b.WriteString("\n" + redStyle.Render(m.err.Error()))
		}

	case phaseDone:
		b.WriteString(greenStyle.Render("Setup complete.") + " Starting knot…")
	}

	return b.String()
}
