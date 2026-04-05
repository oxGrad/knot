package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	stylePending = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// ── pkg status ────────────────────────────────────────────────────────────────

type pkgStatus int

const (
	statusUntied   pkgStatus = iota
	statusTied
	statusPartial
	statusConflict
	statusSkipped
)

func (s pkgStatus) label() string {
	switch s {
	case statusTied:
		return styleGreen.Render("tied   ")
	case statusUntied:
		return styleDim.Render("untied ")
	case statusPartial:
		return styleYellow.Render("partial")
	case statusConflict:
		return styleRed.Render("conflict")
	case statusSkipped:
		return styleDim.Render("skipped")
	}
	return "unknown"
}

func computeStatus(actions []linker.LinkAction) pkgStatus {
	var tied, untied, conflict, skipped int
	for _, a := range actions {
		switch a.Op {
		case linker.OpExists:
			tied++
		case linker.OpCreate:
			untied++
		case linker.OpConflict, linker.OpBroken:
			conflict++
		case linker.OpSkip:
			skipped++
		}
	}
	nonSkip := tied + untied + conflict
	if nonSkip == 0 {
		return statusSkipped
	}
	if conflict > 0 {
		return statusConflict
	}
	if tied > 0 && untied == 0 {
		return statusTied
	}
	if untied > 0 && tied == 0 {
		return statusUntied
	}
	return statusPartial
}

// ── model types ───────────────────────────────────────────────────────────────

type pkgRow struct {
	name    string
	status  pkgStatus
	actions []linker.LinkAction
}

type tuiPhase int

const (
	phaseList     tuiPhase = iota
	phaseConfirm
	phaseApply
	phaseResult
	phaseGitPull
	phaseBranch   // branch picker
	phaseCheckout // checking out a branch
)

type model struct {
	cfg     *config.Config
	cfgPath string
	lnk     *linker.Linker

	rows    []pkgRow
	cursor  int
	offset  int
	toggles map[string]bool

	// git info shown in header
	gitBranch    string
	gitSHA       string
	gitCommitMsg string

	// branch picker state
	branches     []string
	branchCursor int
	branchOffset int

	phase        tuiPhase
	confirmLines []string
	applyLog     []string
	applyErr     error
	statusMsg    string // inline error for editor failure etc.

	width, height int
}

// ── message types ─────────────────────────────────────────────────────────────

type gitInfoMsg struct {
	branch string
	sha    string
	msg    string
	err    error
}

type gitPullResultMsg struct {
	output string
	err    error
}

type reloadMsg struct {
	rows []pkgRow
	cfg  *config.Config
	err  error
}

type applyDoneMsg struct {
	log []string
	err error
}

type editorDoneMsg struct {
	err error
}

type branchListMsg struct {
	branches []string
	err      error
}

type checkoutDoneMsg struct {
	output string
	err    error
}

// ── tab types ─────────────────────────────────────────────────────────────────

type tabKind int

const (
	tabPackages tabKind = iota
	tabTags
)

// ── tag types ─────────────────────────────────────────────────────────────────

type tagRow struct {
	name      string
	status    pkgStatus
	pkgs      []pkgRow
	collapsed bool
}

type tagItem struct {
	isTag       bool
	tag         *tagRow // set when isTag == true
	pkg         *pkgRow // set when isTag == false
	tagName     string  // parent tag name (for package items)
	isLastChild bool    // for package items: is this the last child of its tag?
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildRows(cfg *config.Config, lnk *linker.Linker) ([]pkgRow, error) {
	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]pkgRow, 0, len(names))
	for _, name := range names {
		actions, err := lnk.Plan(cfg, []string{name})
		if err != nil {
			return nil, fmt.Errorf("plan %q: %w", name, err)
		}
		rows = append(rows, pkgRow{
			name:    name,
			status:  computeStatus(actions),
			actions: actions,
		})
	}
	return rows, nil
}

// buildTagRows groups the already-computed pkgRows by tag, sorted by tag name.
// It does not call Plan again — it reuses the rows already built.
func buildTagRows(cfg *config.Config, allRows []pkgRow) []tagRow {
	byTag := config.PackagesByTag(cfg)
	if len(byTag) == 0 {
		return nil
	}

	tagNames := make([]string, 0, len(byTag))
	for name := range byTag {
		tagNames = append(tagNames, name)
	}
	sort.Strings(tagNames)

	rowsByName := make(map[string]pkgRow, len(allRows))
	for _, r := range allRows {
		rowsByName[r.name] = r
	}

	rows := make([]tagRow, 0, len(tagNames))
	for _, tagName := range tagNames {
		pkgNames := byTag[tagName]
		pkgs := make([]pkgRow, 0, len(pkgNames))
		var allActions []linker.LinkAction
		for _, pname := range pkgNames {
			if r, ok := rowsByName[pname]; ok {
				pkgs = append(pkgs, r)
				allActions = append(allActions, r.actions...)
			}
		}
		rows = append(rows, tagRow{
			name:   tagName,
			status: computeStatus(allActions),
			pkgs:   pkgs,
		})
	}
	return rows
}

// visibleTagItems returns the flat list of items currently visible in the Tags tab,
// respecting each tagRow's collapsed state.
func visibleTagItems(rows []tagRow) []tagItem {
	var items []tagItem
	for i := range rows {
		tr := &rows[i]
		items = append(items, tagItem{isTag: true, tag: tr})
		if !tr.collapsed {
			for j := range tr.pkgs {
				items = append(items, tagItem{
					isTag:       false,
					pkg:         &tr.pkgs[j],
					tagName:     tr.name,
					isLastChild: j == len(tr.pkgs)-1,
				})
			}
		}
	}
	return items
}

func seedToggles(rows []pkgRow) map[string]bool {
	t := make(map[string]bool, len(rows))
	for _, r := range rows {
		t[r.name] = r.status == statusTied || r.status == statusPartial
	}
	return t
}

func (m *model) isPending(row pkgRow) bool {
	wantTied := m.toggles[row.name]
	currentlyTied := row.status == statusTied || row.status == statusPartial
	return wantTied != currentlyTied
}

func (m *model) pendingCount() int {
	n := 0
	for _, r := range m.rows {
		if m.isPending(r) {
			n++
		}
	}
	return n
}

func (m *model) togglePackage(i int) {
	row := m.rows[i]
	if row.status == statusSkipped || row.status == statusConflict {
		return
	}
	m.toggles[row.name] = !m.toggles[row.name]
}

// toggleTag bulk-toggles all non-skipped, non-conflict packages in a tag.
// tied → marks all for untie; untied or partial → marks missing for tie.
func (m *model) toggleTag(tr *tagRow) {
	switch tr.status {
	case statusTied:
		for _, pkg := range tr.pkgs {
			if pkg.status == statusSkipped || pkg.status == statusConflict {
				continue
			}
			m.toggles[pkg.name] = false
		}
	case statusUntied:
		for _, pkg := range tr.pkgs {
			if pkg.status == statusSkipped || pkg.status == statusConflict {
				continue
			}
			m.toggles[pkg.name] = true
		}
	case statusPartial:
		for _, pkg := range tr.pkgs {
			if pkg.status == statusSkipped || pkg.status == statusConflict {
				continue
			}
			currentlyTied := pkg.status == statusTied || pkg.status == statusPartial
			if !currentlyTied {
				m.toggles[pkg.name] = true
			}
		}
	}
}

func (m *model) listHeaderLines() int {
	// title + git-info (if available) + divider = 2 or 3
	if m.gitBranch != "" {
		return 3
	}
	return 2
}

func (m *model) visibleHeight() int {
	// header + blank + status + help
	overhead := m.listHeaderLines() + 3
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

func dotfilesDir(cfgPath string) string {
	return filepath.Dir(cfgPath)
}

// ── tea.Cmds ─────────────────────────────────────────────────────────────────

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

// ── bubbletea interface ───────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return fetchGitInfoCmd(dotfilesDir(m.cfgPath))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustOffset()
		m.adjustBranchOffset()
		return m, nil

	case gitInfoMsg:
		if msg.err == nil {
			m.gitBranch = msg.branch
			m.gitSHA = msg.sha
			m.gitCommitMsg = msg.msg
		}
		return m, nil

	case tea.KeyMsg:
		switch m.phase {
		case phaseList:
			return m.updateList(msg)
		case phaseConfirm:
			return m.updateConfirm(msg)
		case phaseBranch:
			return m.updateBranch(msg)
		case phaseResult:
			m.phase = phaseList
			m.applyLog = nil
			m.applyErr = nil
			m.toggles = seedToggles(m.rows)
			return m, nil
		case phaseApply, phaseGitPull, phaseCheckout:
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}

	case gitPullResultMsg:
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = []string{msg.output}
			m.applyErr = fmt.Errorf("git pull: %w", msg.err)
			return m, nil
		}
		dir := dotfilesDir(m.cfgPath)
		return m, tea.Batch(reloadCmd(m.cfgPath, m.lnk), fetchGitInfoCmd(dir))

	case reloadMsg:
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = nil
			m.applyErr = msg.err
			return m, nil
		}
		m.cfg = msg.cfg
		m.rows = msg.rows
		m.toggles = seedToggles(m.rows)
		m.cursor = min(m.cursor, len(m.rows)-1)
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.adjustOffset()
		m.phase = phaseList
		return m, nil

	case applyDoneMsg:
		m.applyLog = msg.log
		m.applyErr = msg.err
		m.phase = phaseResult
		return m, reloadCmd(m.cfgPath, m.lnk)

	case editorDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("editor error: %v", msg.err)
			m.phase = phaseList
			return m, nil
		}
		m.statusMsg = ""
		return m, reloadCmd(m.cfgPath, m.lnk)

	case branchListMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("git branch: %v", msg.err)
			return m, nil
		}
		m.branches = msg.branches
		m.branchCursor = 0
		for i, b := range m.branches {
			if b == m.gitBranch {
				m.branchCursor = i
				break
			}
		}
		m.branchOffset = 0
		m.adjustBranchOffset()
		m.phase = phaseBranch
		return m, nil

	case checkoutDoneMsg:
		if msg.err != nil {
			m.phase = phaseResult
			m.applyLog = []string{strings.TrimSpace(msg.output)}
			m.applyErr = fmt.Errorf("git checkout: %w", msg.err)
			return m, nil
		}
		dir := dotfilesDir(m.cfgPath)
		return m, tea.Batch(reloadCmd(m.cfgPath, m.lnk), fetchGitInfoCmd(dir))
	}

	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.adjustOffset()
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.adjustOffset()
		}
	case " ":
		m.togglePackage(m.cursor)
	case "a":
		if m.pendingCount() == 0 {
			break
		}
		m.confirmLines = m.buildConfirmLines()
		m.phase = phaseConfirm
	case "r":
		m.phase = phaseGitPull
		return m, gitPullCmd(m.cfgPath)
	case "b":
		return m, fetchBranchesCmd(dotfilesDir(m.cfgPath))
	case "e":
		return m, editorCmd(m.cfgPath)
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		m.phase = phaseApply
		return m, applyCmd(m.cfg, m.lnk, m.rows, m.toggles)
	case "n", "esc", "q":
		m.phase = phaseList
		m.confirmLines = nil
	}
	return m, nil
}

func (m model) updateBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
			m.adjustBranchOffset()
		}
	case "down", "j":
		if m.branchCursor < len(m.branches)-1 {
			m.branchCursor++
			m.adjustBranchOffset()
		}
	case "enter":
		selected := m.branches[m.branchCursor]
		if selected == m.gitBranch {
			m.phase = phaseList // already on this branch
			break
		}
		m.phase = phaseCheckout
		return m, checkoutBranchCmd(dotfilesDir(m.cfgPath), selected)
	case "esc", "q":
		m.phase = phaseList
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) buildConfirmLines() []string {
	var lines []string
	for _, row := range m.rows {
		if !m.isPending(row) {
			continue
		}
		if m.toggles[row.name] {
			lines = append(lines, fmt.Sprintf("  tie   %s", row.name))
		} else {
			lines = append(lines, fmt.Sprintf("  untie %s", row.name))
		}
	}
	return lines
}

// ── views ─────────────────────────────────────────────────────────────────────

func (m model) View() string {
	switch m.phase {
	case phaseConfirm:
		return m.viewConfirm()
	case phaseApply:
		return styleDim.Render("Applying changes...")
	case phaseResult:
		return m.viewResult()
	case phaseGitPull:
		return styleDim.Render(fmt.Sprintf("Running git pull in %s...", dotfilesDir(m.cfgPath)))
	case phaseCheckout:
		return styleDim.Render(fmt.Sprintf("Switching branch in %s...", dotfilesDir(m.cfgPath)))
	case phaseBranch:
		return m.viewBranch()
	default:
		return m.viewList()
	}
}

func (m model) viewList() string {
	var b strings.Builder

	// Header
	b.WriteString(styleBold.Render("knot") + styleDim.Render(" — interactive mode") + "\n")
	if m.gitBranch != "" {
		commitInfo := m.gitSHA
		if m.gitCommitMsg != "" {
			maxMsgLen := max(m.width-len(m.gitBranch)-len(m.gitSHA)-10, 20)
			msg := m.gitCommitMsg
			if len(msg) > maxMsgLen {
				msg = msg[:maxMsgLen-1] + "…"
			}
			commitInfo = m.gitSHA + " " + msg
		}
		b.WriteString(styleDim.Render("on ") + styleCyan.Render(m.gitBranch) + styleDim.Render(" · "+commitInfo) + "\n")
	}
	b.WriteString(strings.Repeat("─", max(m.width, 30)) + "\n")

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

		for i := m.offset; i < end; i++ {
			row := m.rows[i]

			cursor := "  "
			if i == m.cursor {
				cursor = styleCursor.Render("▶ ")
			}

			name := fmt.Sprintf("%-*s", nameWidth, row.name)

			pending := "  "
			if m.isPending(row) {
				pending = stylePending.Render(" *")
			}

			b.WriteString(fmt.Sprintf("%s%s  [%s]%s\n", cursor, name, row.status.label(), pending))
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

func (m model) viewBranch() string {
	var b strings.Builder

	title := styleBold.Render("Switch branch")
	if m.gitBranch != "" {
		title += styleDim.Render("  (on ") + styleCyan.Render(m.gitBranch) + styleDim.Render(")")
	}
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat("─", max(m.width, 30)) + "\n")

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

			b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, current))
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
		return fmt.Errorf("loading config: %w", err)
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
		phase:   phaseList,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
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
