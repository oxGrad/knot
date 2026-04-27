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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
	"github.com/spf13/cobra"
)

// ── styles ────────────────────────────────────────────────────────────────────

const (
	tuiMarginLeft  = 4
	tuiMarginRight = 4
)

var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	stylePending = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleMargin  = lipgloss.NewStyle().PaddingLeft(tuiMarginLeft).PaddingRight(tuiMarginRight)

	// header ASCII art gradient: light-green (top) → dark-green (bottom)
	styleArt = [6]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("83")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("40")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("28")).Bold(true),
	}
	styleMascotNormal   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleMascotConflict = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleMascotMissing  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// ── pkg status ────────────────────────────────────────────────────────────────

type pkgStatus int

const (
	statusUntied pkgStatus = iota
	statusTied
	statusPartial
	statusConflict
	statusSkipped
	statusSourceNotFound
)

const statusWidth = 9 // width of the widest label ("no source")

func centerLabel(s string) string {
	pad := statusWidth - len(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func (s pkgStatus) label() string {
	switch s {
	case statusTied:
		return styleGreen.Render(centerLabel("tied"))
	case statusUntied:
		return styleDim.Render(centerLabel("untied"))
	case statusPartial:
		return styleYellow.Render(centerLabel("partial"))
	case statusConflict:
		return styleRed.Render(centerLabel("conflict"))
	case statusSkipped:
		return styleDim.Render(centerLabel("skipped"))
	case statusSourceNotFound:
		return styleYellow.Render(centerLabel("no source"))
	}
	return centerLabel("unknown")
}

func computeStatus(actions []linker.LinkAction) pkgStatus {
	var tied, untied, conflict, skipped, sourceNotFound int
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
		case linker.OpSourceNotFound:
			sourceNotFound++
		}
	}
	nonSkip := tied + untied + conflict
	if nonSkip == 0 && sourceNotFound > 0 {
		return statusSourceNotFound
	}
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

// ── header art & mascot ───────────────────────────────────────────────────────

// knotArt is "KNOT" in 6-row block-letter style; each row is 37 visual columns wide.
var knotArt = [6]string{
	`██╗  ██╗███╗   ██╗ ██████╗ ████████╗`,
	`██║ ██╔╝████╗  ██║██╔═══██╗╚══██╔══╝`,
	`█████╔╝ ██╔██╗ ██║██║   ██║   ██║   `,
	`██╔═██╗ ██║╚██╗██║██║   ██║   ██║   `,
	`██║  ██╗██║ ╚████║╚██████╔╝   ██║   `,
	`╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝    ╚═╝   `,
}

type mascotState int

const (
	mascotNormal   mascotState = iota // idle
	mascotConflict                    // package conflict detected
	mascotMissing                     // no packages / no git repo
)

// mascotFrames[state][frame][line] — each line is exactly 8 visual columns.
var mascotFrames = [3][3][6]string{
	// mascotNormal: green, slow blink
	{
		{` ▄████▄ `, ` █ oo █ `, ` ▀████▀ `, `  ████  `, ` ██  ██ `, `██    ██`},
		{` ▄████▄ `, ` █ -- █ `, ` ▀████▀ `, `  ████  `, ` ██  ██ `, `██    ██`},
		{` ▄████▄ `, ` █ oo █ `, ` ▀████▀ `, `▐ ████ ▌`, ` ██  ██ `, `██    ██`},
	},
	// mascotConflict: red, frantic
	{
		{` ▄████▄ `, ` █ !! █ `, ` ▀████▀ `, `  ████  `, ` ██  ██ `, ` /    \ `},
		{` ▄████▄ `, ` █ ** █ `, ` ▀████▀ `, `  ████  `, `▌██  ██▐`, ` \    / `},
		{` ▄████▄ `, ` █ XX █ `, ` ▀████▀ `, `  ████  `, ` ██  ██ `, `▌/    \▐`},
	},
	// mascotMissing: yellow, looking side-to-side
	{
		{` ▄████▄ `, ` █ ?? █ `, ` ▀████▀ `, `   ██   `, `  ████  `, `  █  █  `},
		{` ▄████▄ `, ` █??   █`, ` ▀████▀ `, `   ██   `, ` ████   `, ` █  █   `},
		{` ▄████▄ `, ` █   ??█`, ` ▀████▀ `, `   ██   `, `   ████ `, `   █  █ `},
	},
}

// ── model types ───────────────────────────────────────────────────────────────

type pkgRow struct {
	name    string
	status  pkgStatus
	actions []linker.LinkAction
}

type tuiPhase int

const (
	phaseList tuiPhase = iota
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

	activeTab tabKind
	tagRows   []tagRow
	tagCursor int
	tagOffset int

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
	headerFrame   int // incremented every 600ms for mascot animation
}

// ── message types ─────────────────────────────────────────────────────────────

type headerTickMsg struct{}

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

// ── pending helpers ───────────────────────────────────────────────────────────

func (m *model) pkgPendingArrow(row pkgRow) string {
	if !m.isPending(row) {
		return ""
	}
	target := "untied"
	if m.toggles[row.name] {
		target = "tied"
	}
	return stylePending.Render(" -> " + target)
}

// tagWouldBeWord returns the word describing the aggregate state a tag would
// reach if all current toggles were applied.
func (m *model) tagWouldBeWord(tr *tagRow) string {
	var tied, untied int
	for _, pkg := range tr.pkgs {
		if pkg.status == statusSkipped || pkg.status == statusConflict || pkg.status == statusSourceNotFound {
			continue
		}
		if m.toggles[pkg.name] {
			tied++
		} else {
			untied++
		}
	}
	if tied > 0 && untied == 0 {
		return "tied"
	}
	if untied > 0 && tied == 0 {
		return "untied"
	}
	return "partial"
}

func (m *model) tagPendingArrow(tr *tagRow) string {
	for _, pkg := range tr.pkgs {
		if m.isPending(pkg) {
			return stylePending.Render(" -> " + m.tagWouldBeWord(tr))
		}
	}
	return ""
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
	if row.status == statusSkipped || row.status == statusConflict || row.status == statusSourceNotFound {
		return
	}
	m.toggles[row.name] = !m.toggles[row.name]
}

// toggleTag bulk-toggles all non-skipped, non-conflict packages in a tag.
// tied → marks all for untie; untied or partial → marks missing for tie.
// If the pending state already matches what the toggle would set, it reverts to seed state.
func (m *model) toggleTag(tr *tagRow) {
	eligible := func(pkg pkgRow) bool {
		return pkg.status != statusSkipped && pkg.status != statusConflict
	}
	switch tr.status {
	case statusTied:
		// Check if already all pending-untie; if so, revert.
		allPending := true
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			if m.toggles[pkg.name] {
				allPending = false
				break
			}
		}
		want := false
		if allPending {
			want = true // revert to seed (tied → wantTied = true)
		}
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			m.toggles[pkg.name] = want
		}
	case statusUntied:
		// Check if already all pending-tie; if so, revert.
		allPending := true
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			if !m.toggles[pkg.name] {
				allPending = false
				break
			}
		}
		want := true
		if allPending {
			want = false // revert to seed (untied → wantTied = false)
		}
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			m.toggles[pkg.name] = want
		}
	case statusPartial:
		// Tie any untied packages. If all untied pkgs are already pending-tie, revert them.
		allPending := true
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			currentlyTied := pkg.status == statusTied || pkg.status == statusPartial
			if !currentlyTied && !m.toggles[pkg.name] {
				allPending = false
				break
			}
		}
		for _, pkg := range tr.pkgs {
			if !eligible(pkg) {
				continue
			}
			currentlyTied := pkg.status == statusTied || pkg.status == statusPartial
			if !currentlyTied {
				if allPending {
					m.toggles[pkg.name] = false // revert to seed
				} else {
					m.toggles[pkg.name] = true
				}
			}
		}
	}
}

func (m *model) listHeaderLines() int {
	// brand box (11 lines) + tab header (1 line) = 12
	return 12
}

func (m *model) visibleHeight() int {
	// header + blank line + list + blank + status + help
	overhead := m.listHeaderLines() + 4
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
	// header + git? + divider + tab header + blank + status + help = listHeaderLines + 4
	overhead := m.listHeaderLines() + 4
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
	mascotLines := mascotFrames[state][frame]

	var mascotStyle lipgloss.Style
	switch state {
	case mascotConflict:
		mascotStyle = styleMascotConflict
	case mascotMissing:
		mascotStyle = styleMascotMissing
	default:
		mascotStyle = styleMascotNormal
	}

	const leftPad = 2
	const gap = 4

	innerW := max(m.width-tuiMarginLeft-tuiMarginRight, 62) - 2
	hLine := strings.Repeat("─", innerW)

	var b strings.Builder

	// top border
	b.WriteString("╭" + hLine + "╮\n")
	// empty line
	b.WriteString("│" + strings.Repeat(" ", innerW) + "│\n")
	// 6 lines of KNOT art + knotman side-by-side; pad each row individually
	// so mismatched art/mascot visual widths don't break the right border.
	for i := 0; i < 6; i++ {
		art := styleArt[i].Render(knotArt[i])
		mascot := mascotStyle.Render(mascotLines[i])
		content := "  " + art + strings.Repeat(" ", gap) + mascot
		rowRightPad := strings.Repeat(" ", max(innerW-lipgloss.Width(content), 0))
		b.WriteString("│" + content + rowRightPad + "│\n")
	}
	// empty line
	b.WriteString("│" + strings.Repeat(" ", innerW) + "│\n")
	// subtitle / git info
	subtitle := styleDim.Render("dotfiles manager")
	if m.gitBranch != "" {
		commitInfo := m.gitSHA
		if m.gitCommitMsg != "" {
			// reserve space for "dotfiles manager · on <branch> · <sha> "
			overhead := len("dotfiles manager · on  · ") + len(m.gitBranch) + len(m.gitSHA) + 1
			maxMsgLen := max(innerW-leftPad-overhead, 10)
			msg := []rune(m.gitCommitMsg)
			if len(msg) > maxMsgLen {
				msg = append(msg[:maxMsgLen-1], '…')
			}
			commitInfo = m.gitSHA + " " + string(msg)
		}
		subtitle += styleDim.Render(" · on ") + styleCyan.Render(m.gitBranch) + styleDim.Render(" · "+commitInfo)
	}
	subtitleVisW := lipgloss.Width(subtitle)
	subRightPad := strings.Repeat(" ", max(innerW-leftPad-subtitleVisW, 0))
	b.WriteString("│  " + subtitle + subRightPad + "│\n")
	// bottom border
	b.WriteString("╰" + hLine + "╯\n")

	return b.String()
}

func (m model) renderTabHeader() string {
	var pkgTab, tagTab string
	if m.activeTab == tabPackages {
		pkgTab = styleBold.Render("Packages")
		tagTab = styleDim.Render("Tags")
	} else {
		pkgTab = styleDim.Render("Packages")
		tagTab = styleBold.Render("Tags")
	}
	return " " + pkgTab + styleDim.Render(" │ ") + tagTab
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

func dotfilesDir(cfgPath string) string {
	return filepath.Dir(cfgPath)
}

// ── tea.Cmds ─────────────────────────────────────────────────────────────────

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

// ── bubbletea interface ───────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return tea.Batch(
		fetchGitInfoCmd(dotfilesDir(m.cfgPath)),
		headerTickCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustOffset()
		m.adjustBranchOffset()
		return m, nil

	case headerTickMsg:
		m.headerFrame++
		return m, headerTickCmd()

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
			if m.activeTab == tabTags {
				return m.updateTags(msg)
			}
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

		// Rebuild tag rows, preserving each tag's collapsed state.
		newTagRows := buildTagRows(msg.cfg, msg.rows)
		collapsedState := make(map[string]bool, len(m.tagRows))
		for _, tr := range m.tagRows {
			collapsedState[tr.name] = tr.collapsed
		}
		for i := range newTagRows {
			if c, ok := collapsedState[newTagRows[i].name]; ok {
				newTagRows[i].collapsed = c
			}
		}
		m.tagRows = newTagRows

		// Clamp both cursors.
		m.cursor = min(m.cursor, len(m.rows)-1)
		if m.cursor < 0 {
			m.cursor = 0
		}
		newVisible := visibleTagItems(m.tagRows)
		m.tagCursor = min(m.tagCursor, len(newVisible)-1)
		if m.tagCursor < 0 {
			m.tagCursor = 0
		}
		m.adjustOffset()
		m.adjustTagOffset()
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
	case "]":
		m.activeTab = tabTags
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

func (m model) updateTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := visibleTagItems(m.tagRows)
	switch msg.String() {
	case "up", "k":
		if m.tagCursor > 0 {
			m.tagCursor--
			m.adjustTagOffset()
		}
	case "down", "j":
		if m.tagCursor < len(items)-1 {
			m.tagCursor++
			m.adjustTagOffset()
		}
	case " ":
		if m.tagCursor < len(items) {
			item := items[m.tagCursor]
			if item.isTag {
				m.toggleTag(item.tag)
			} else {
				for i, r := range m.rows {
					if r.name == item.pkg.name {
						m.togglePackage(i)
						break
					}
				}
			}
		}
	case "enter":
		if m.tagCursor < len(items) {
			item := items[m.tagCursor]
			if item.isTag {
				item.tag.collapsed = !item.tag.collapsed
				m.adjustTagOffset()
			}
		}
	case "a":
		if m.pendingCount() == 0 {
			break
		}
		m.confirmLines = m.buildConfirmLines()
		m.phase = phaseConfirm
	case "[":
		m.activeTab = tabPackages
	case "q", "ctrl+c":
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
	b.WriteString(m.renderTabHeader() + "\n")
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
	b.WriteString(m.renderTabHeader() + "\n")
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
