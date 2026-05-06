package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)


func (m *model) listHeaderLines() int {
	// blank + brand box (13 lines, tabs embedded in bottom border) = 14
	return 14
}

func (m *model) visibleHeight() int {
	// header + blank line + top indicator + list + bottom indicator + blank + status + help
	overhead := m.listHeaderLines() + 6
	v := m.height - overhead
	if v < 1 {
		return 1
	}
	return v
}

func (m *model) adjustOffset() {
	visibleRows := m.visibleHeight()
	if visibleRows <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
}

func (m *model) branchVisibleHeight() int {
	// title + divider + blank + help = 4
	v := m.height - 4
	if v < 1 {
		return 1
	}
	return v
}

func (m *model) adjustBranchOffset() {
	vh := m.branchVisibleHeight()
	if vh <= 0 {
		return
	}
	if m.branchCursor < m.branchOffset {
		m.branchOffset = m.branchCursor
	}
	if m.branchCursor >= m.branchOffset+vh {
		m.branchOffset = m.branchCursor - vh + 1
	}
}

func (m *model) tagVisibleHeight() int {
	// header + blank + top indicator + list + bottom indicator + blank + status + help
	overhead := m.listHeaderLines() + 6
	v := m.height - overhead
	if v < 1 {
		return 1
	}
	return v
}

func (m *model) adjustTagOffset() {
	items := visibleTagItems(m.tagRows)
	vh := m.tagVisibleHeight()
	if vh <= 0 || len(items) == 0 {
		return
	}
	if m.tagCursor < m.tagOffset {
		m.tagOffset = m.tagCursor
	}
	if m.tagCursor >= m.tagOffset+vh {
		m.tagOffset = m.tagCursor - vh + 1
	}
	maxOffset := max(0, len(items)-vh)
	if m.tagOffset > maxOffset {
		m.tagOffset = maxOffset
	}
}

func (m model) renderBrandHeader() string {
	state := m.currentMascotState()
	var frame int
	if state == mascotNormal {
		frame = (m.headerFrame / 2) % 3
	} else {
		frame = m.headerFrame % 3
	}
	var frames [3][3][6]string
	switch m.mascotChar {
	case mascotMonkey:
		frames = monkeyFrames
	case mascotRobot:
		frames = robotFrames
	default:
		frames = jellyfishFrames
	}
	mascotLines := frames[state][frame]

	var mascotStyle lipgloss.Style
	switch state {
	case mascotConflict:
		mascotStyle = styleMascotConflict
	case mascotMissing:
		mascotStyle = styleMascotMissing
	default:
		switch m.mascotChar {
		case mascotRobot:
			mascotStyle = styleMascotRobotNormal
		case mascotJellyfish:
			mascotStyle = styleMascotJellyNormal
		default:
			mascotStyle = styleMascotNormal
		}
	}

	const gap = 4
	// left section: 4 spaces + art (37) + gap (4) + mascot (8) + 3 padding = 56
	const divX = 56

	innerW := max(m.width-tuiMarginLeft-tuiMarginRight, 62) - 2
	rightW := max(innerW-divX-1, 0)

	var b strings.Builder

	b.WriteString("\n")
	// top border: ╭── Knot v0.1.0 ──...──┬──...──╮
	titleName := styleBorder.Render("Knot")
	titleVersion := styleDim.Render(" v" + Version)
	titleSegment := " " + titleName + titleVersion + " "
	titleSegmentW := lipgloss.Width(titleSegment)
	leftTopFill := styleBorder.Render("──") + titleSegment + styleBorder.Render(strings.Repeat("─", max(divX-2-titleSegmentW, 0)))
	rightTopFill := styleBorder.Render(strings.Repeat("─", rightW))
	b.WriteString(styleBorder.Render("╭") + leftTopFill + styleBorder.Render("┬") + rightTopFill + styleBorder.Render("╮") + "\n")

	// build right panel rows (top pad + welcome + spacer + 6 art + 1 empty + 1 bottom = 11)
	// indices: top=0, welcome=1, spacer=2, art[0..5]=3..8, empty=9, bottom=10
	rightRows := make([]string, 11)
	fill := func(s string) string { return s + strings.Repeat(" ", max(rightW-lipgloss.Width(s), 0)) }
	for i := range rightRows {
		rightRows[i] = strings.Repeat(" ", rightW)
	}
	if m.gitBranch != "" {
		rightRows[4] = fill(" " + styleDim.Render("branch  ") + styleCyan.Render(m.gitBranch))
	}
	if m.gitSHA != "" {
		rightRows[5] = fill(" " + styleDim.Render("commit  ") + styleDim.Render(m.gitSHA))
	}
	if m.gitCommitMsg != "" {
		maxMsgLen := max(rightW-len(" message  ")-1, 5)
		msg := []rune(m.gitCommitMsg)
		if len(msg) > maxMsgLen {
			msg = append(msg[:maxMsgLen-1], '…')
		}
		rightRows[6] = fill(" " + styleDim.Render("message ") + string(msg))
	}

	writeRow := func(left, right string) {
		b.WriteString(styleBorder.Render("│") + left + styleBorder.Render("│") + right + styleBorder.Render("│") + "\n")
	}

	// top padding
	writeRow(strings.Repeat(" ", divX), rightRows[0])
	// welcome line
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("LOGNAME")
	}
	welcomeLeft := "    " + styleDim.Render("Welcome, ") + username + "!"
	welcomeLeft += strings.Repeat(" ", max(divX-lipgloss.Width(welcomeLeft), 0))
	writeRow(welcomeLeft, rightRows[1])
	// spacer between welcome and art
	writeRow(strings.Repeat(" ", divX), rightRows[2])
	// 6 art rows
	for i := 0; i < 6; i++ {
		art := styleArt[i].Render(knotArt[i])
		mascot := renderMascotLine(mascotLines[i], mascotStyle)
		leftContent := "    " + art + strings.Repeat(" ", gap) + mascot
		leftContent += strings.Repeat(" ", max(divX-lipgloss.Width(leftContent), 0))
		writeRow(leftContent, rightRows[3+i])
	}
	// empty bottom line + subtitle row
	writeRow(strings.Repeat(" ", divX), rightRows[9])
	writeRow(strings.Repeat(" ", divX), rightRows[10])

	// bottom border with embedded tabs: ╰── Packages │ Tags ──...──┴──...──╯
	var pkgTab, tagTab string
	if m.activeTab == tabPackages {
		pkgTab = styleBold.Render("Packages")
		tagTab = styleDim.Render("Tags")
	} else {
		pkgTab = styleDim.Render("Packages")
		tagTab = styleBold.Render("Tags")
	}
	tabSegment := " " + pkgTab + styleDim.Render(" · ") + tagTab + "  "
	tabSegmentW := lipgloss.Width(tabSegment)
	leftBottomFill := styleBorder.Render("── ") + tabSegment + styleBorder.Render(strings.Repeat("─", max(divX-3-tabSegmentW, 0)))
	b.WriteString(styleBorder.Render("╰") + leftBottomFill + styleBorder.Render("┴") + styleBorder.Render(strings.Repeat("─", rightW)) + styleBorder.Render("╯") + "\n")

	return b.String()
}

func (m model) hasConflicts() bool {
	for _, r := range m.rows {
		if r.status == statusConflict {
			return true
		}
	}
	return false
}

func (m model) currentMascotState() mascotState {
	if m.hasConflicts() {
		return mascotConflict
	}
	if len(m.rows) == 0 || m.gitBranch == "" {
		return mascotMissing
	}
	return mascotNormal
}

// ── bubbletea interface ───────────────────────────────────────────────────────

// ── views ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	var v string
	switch m.phase {
	case phaseConfirm:
		v = m.viewConfirm()
	case phaseApply:
		v = styleDim.Render("Applying changes...")
	case phaseResult:
		v = m.viewResult()
	case phaseGitPull:
		v = styleDim.Render(fmt.Sprintf("Running git pull in %s...", dotfilesDir(m.cfgPath)))
	case phaseCheckout:
		v = styleDim.Render(fmt.Sprintf("Switching branch in %s...", dotfilesDir(m.cfgPath)))
	case phaseBranch:
		v = m.viewBranch()
	default:
		if m.activeTab == tabTags {
			v = m.viewTags()
		} else {
			v = m.viewList()
		}
	}
	return styleMargin.Render(v)
}

func (m model) viewList() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderBrandHeader())
	b.WriteString("\n")

	// Package list
	if len(m.rows) == 0 {
		b.WriteString(styleDim.Render("No packages defined in Knotfile.") + "\n")
	} else {
		nameWidth := 0
		for _, r := range m.rows {
			if len(r.name) > nameWidth {
				nameWidth = len(r.name)
			}
		}

		visibleRows := m.visibleHeight()
		end := m.offset + visibleRows
		if end > len(m.rows) {
			end = len(m.rows)
		}

		if m.offset > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↑ %d more", m.offset)) + "\n")
		} else {
			b.WriteString("\n")
		}

		for i := m.offset; i < end; i++ {
			row := m.rows[i]

			cursor := "  "
			if i == m.cursor {
				cursor = styleCursor.Render("▶ ")
			}

			name := fmt.Sprintf("%-*s", nameWidth, row.name)

			pending := m.pkgPendingArrow(row)

			fmt.Fprintf(&b, "%s  %s  [%s]%s\n", cursor, name, row.status.label(), pending)
		}

		below := len(m.rows) - end
		if below > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↓ %d more", below)) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Status / error line
	if m.statusMsg != "" {
		b.WriteString(styleRed.Render(m.statusMsg) + "\n")
	} else if pc := m.pendingCount(); pc > 0 {
		b.WriteString(styleYellow.Render(fmt.Sprintf("%d pending change(s)", pc)) + "\n")
	} else {
		b.WriteString(styleDim.Render("No pending changes") + "\n")
	}

	b.WriteString(styleDim.Render("↑↓/jk navigate · space toggle · a apply · b branch · r pull · e edit · q quit"))

	return b.String()
}

func (m model) viewTags() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderBrandHeader())
	b.WriteString("\n")

	items := visibleTagItems(m.tagRows)
	if len(items) == 0 {
		b.WriteString(styleDim.Render("No tagged packages defined.") + "\n")
	} else {
		// Compute name width: max of tag names and (package names + tree prefix).
		nameWidth := 0
		for _, tr := range m.tagRows {
			if len(tr.name) > nameWidth {
				nameWidth = len(tr.name)
			}
			for _, pkg := range tr.pkgs {
				// indent + connector = 7 chars ("  ├── " or "  └── ")
				if len(pkg.name)+7 > nameWidth {
					nameWidth = len(pkg.name) + 7
				}
			}
		}

		vh := m.tagVisibleHeight()
		end := m.tagOffset + vh
		if end > len(items) {
			end = len(items)
		}

		if m.tagOffset > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↑ %d more", m.tagOffset)) + "\n")
		} else {
			b.WriteString("\n")
		}

		for i := m.tagOffset; i < end; i++ {
			item := items[i]
			cursor := "  "
			if i == m.tagCursor {
				cursor = styleCursor.Render("▶ ")
			}

			if item.isTag {
				collapsePrefix := "  "
				if item.tag.collapsed {
					collapsePrefix = styleDim.Render("▶ ")
				}
				pendingMark := m.tagPendingArrow(item.tag)
				name := fmt.Sprintf("%-*s", nameWidth, item.tag.name)
				fmt.Fprintf(&b, "%s%s%s  [%s]%s\n",
					cursor, collapsePrefix,
					styleCyan.Render(styleBold.Render(name)),
					item.tag.status.label(), pendingMark)
			} else {
				connector := "├── "
				if item.isLastChild {
					connector = "└── "
				}
				pkgName := fmt.Sprintf("%-*s", nameWidth-7, item.pkg.name)
				pendingMark := m.pkgPendingArrow(*item.pkg)
				fmt.Fprintf(&b, "%s  %s  [%s]%s\n",
					cursor,
					styleDim.Render(connector+pkgName),
					item.pkg.status.label(), pendingMark)
			}
		}

		below := len(items) - end
		if below > 0 {
			b.WriteString(styleDim.Render(fmt.Sprintf("  ↓ %d more", below)) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(styleRed.Render(m.statusMsg) + "\n")
	} else if pc := m.pendingCount(); pc > 0 {
		b.WriteString(styleYellow.Render(fmt.Sprintf("%d pending change(s)", pc)) + "\n")
	} else {
		b.WriteString(styleDim.Render("No pending changes") + "\n")
	}
	b.WriteString(styleDim.Render("↑↓/jk navigate · space toggle · enter collapse · a apply · [ ] tabs · q quit"))
	return b.String()
}

func (m model) viewBranch() string {
	var b strings.Builder

	title := styleBold.Render("Switch branch")
	if m.gitBranch != "" {
		title += styleDim.Render("  (on ") + styleCyan.Render(m.gitBranch) + styleDim.Render(")")
	}
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", max(m.width-tuiMarginLeft-tuiMarginRight, 30)) + "\n")

	if len(m.branches) == 0 {
		b.WriteString(styleDim.Render("No branches found.") + "\n")
	} else {
		nameWidth := 0
		for _, br := range m.branches {
			if len(br) > nameWidth {
				nameWidth = len(br)
			}
		}

		vh := m.branchVisibleHeight()
		end := m.branchOffset + vh
		if end > len(m.branches) {
			end = len(m.branches)
		}

		for i := m.branchOffset; i < end; i++ {
			br := m.branches[i]

			cursor := "  "
			if i == m.branchCursor {
				cursor = styleCursor.Render("▶ ")
			}

			name := fmt.Sprintf("%-*s", nameWidth, br)

			current := ""
			if br == m.gitBranch {
				current = "  " + styleDim.Render("(current)")
			}

			fmt.Fprintf(&b, "%s%s%s\n", cursor, name, current)
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("↑↓/jk navigate · enter switch · esc cancel"))

	return b.String()
}

func (m model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(styleBold.Render("Pending changes:") + "\n")
	for _, line := range m.confirmLines {
		b.WriteString(styleCyan.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleBold.Render("Apply? [y/n]"))
	return b.String()
}

func (m model) viewResult() string {
	var b strings.Builder
	if m.applyErr != nil {
		b.WriteString(styleRed.Render("Error:") + " " + m.applyErr.Error() + "\n")
	} else {
		b.WriteString(styleGreen.Render("Done.") + "\n")
	}
	for _, line := range m.applyLog {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("Press any key to return."))
	return b.String()
}

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
