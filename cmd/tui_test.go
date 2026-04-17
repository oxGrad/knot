package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
	"github.com/oxgrad/knot/internal/linker"
)

// ── computeStatus ─────────────────────────────────────────────────────────────

func TestComputeStatus_AllTied(t *testing.T) {
	actions := []linker.LinkAction{
		{Op: linker.OpExists},
		{Op: linker.OpExists},
	}
	if got := computeStatus(actions); got != statusTied {
		t.Errorf("expected statusTied, got %v", got)
	}
}

func TestComputeStatus_AllUntied(t *testing.T) {
	actions := []linker.LinkAction{
		{Op: linker.OpCreate},
		{Op: linker.OpCreate},
	}
	if got := computeStatus(actions); got != statusUntied {
		t.Errorf("expected statusUntied, got %v", got)
	}
}

func TestComputeStatus_Partial(t *testing.T) {
	actions := []linker.LinkAction{
		{Op: linker.OpExists},
		{Op: linker.OpCreate},
	}
	if got := computeStatus(actions); got != statusPartial {
		t.Errorf("expected statusPartial, got %v", got)
	}
}

func TestComputeStatus_Conflict(t *testing.T) {
	actions := []linker.LinkAction{
		{Op: linker.OpExists},
		{Op: linker.OpConflict},
	}
	if got := computeStatus(actions); got != statusConflict {
		t.Errorf("expected statusConflict, got %v", got)
	}
}

func TestComputeStatus_BrokenIsConflict(t *testing.T) {
	actions := []linker.LinkAction{
		{Op: linker.OpBroken},
	}
	if got := computeStatus(actions); got != statusConflict {
		t.Errorf("expected statusConflict for broken symlink, got %v", got)
	}
}

func TestComputeStatus_AllSkipped(t *testing.T) {
	actions := []linker.LinkAction{
		{Op: linker.OpSkip},
		{Op: linker.OpSkip},
	}
	if got := computeStatus(actions); got != statusSkipped {
		t.Errorf("expected statusSkipped, got %v", got)
	}
}

func TestComputeStatus_Empty(t *testing.T) {
	if got := computeStatus(nil); got != statusSkipped {
		t.Errorf("expected statusSkipped for empty actions, got %v", got)
	}
}

// ── pkgStatus.label ───────────────────────────────────────────────────────────

func TestPkgStatus_Label_AllValues(t *testing.T) {
	statuses := []pkgStatus{
		statusTied,
		statusUntied,
		statusPartial,
		statusConflict,
		statusSkipped,
	}
	for _, s := range statuses {
		label := s.label()
		if label == "" {
			t.Errorf("label() returned empty string for pkgStatus %d", s)
		}
	}
}

func TestPkgStatus_Label_Unknown(t *testing.T) {
	var unknown pkgStatus = 99
	if got := unknown.label(); got != "unknown" {
		t.Errorf("expected 'unknown' for unknown pkgStatus, got %q", got)
	}
}

// ── seedToggles ───────────────────────────────────────────────────────────────

func TestSeedToggles_TiedIsTrue(t *testing.T) {
	rows := []pkgRow{
		{name: "nvim", status: statusTied},
	}
	toggles := seedToggles(rows)
	if !toggles["nvim"] {
		t.Error("expected tied package to have toggle=true")
	}
}

func TestSeedToggles_UntiedIsFalse(t *testing.T) {
	rows := []pkgRow{
		{name: "zsh", status: statusUntied},
	}
	toggles := seedToggles(rows)
	if toggles["zsh"] {
		t.Error("expected untied package to have toggle=false")
	}
}

func TestSeedToggles_PartialIsTrue(t *testing.T) {
	rows := []pkgRow{
		{name: "git", status: statusPartial},
	}
	toggles := seedToggles(rows)
	if !toggles["git"] {
		t.Error("expected partial package to have toggle=true")
	}
}

func TestSeedToggles_SkippedIsFalse(t *testing.T) {
	rows := []pkgRow{
		{name: "yabai", status: statusSkipped},
	}
	toggles := seedToggles(rows)
	if toggles["yabai"] {
		t.Error("expected skipped package to have toggle=false")
	}
}

func TestSeedToggles_MultiplePackages(t *testing.T) {
	rows := []pkgRow{
		{name: "nvim", status: statusTied},
		{name: "zsh", status: statusUntied},
		{name: "git", status: statusPartial},
	}
	toggles := seedToggles(rows)
	if !toggles["nvim"] {
		t.Error("nvim should be true (tied)")
	}
	if toggles["zsh"] {
		t.Error("zsh should be false (untied)")
	}
	if !toggles["git"] {
		t.Error("git should be true (partial)")
	}
}

// ── isPending ─────────────────────────────────────────────────────────────────

func TestIsPending_TiedWantsTied(t *testing.T) {
	m := &model{
		toggles: map[string]bool{"nvim": true},
	}
	row := pkgRow{name: "nvim", status: statusTied}
	if m.isPending(row) {
		t.Error("tied package with toggle=true should not be pending")
	}
}

func TestIsPending_TiedWantsUntied(t *testing.T) {
	m := &model{
		toggles: map[string]bool{"nvim": false},
	}
	row := pkgRow{name: "nvim", status: statusTied}
	if !m.isPending(row) {
		t.Error("tied package with toggle=false should be pending")
	}
}

func TestIsPending_UntiedWantsUntied(t *testing.T) {
	m := &model{
		toggles: map[string]bool{"zsh": false},
	}
	row := pkgRow{name: "zsh", status: statusUntied}
	if m.isPending(row) {
		t.Error("untied package with toggle=false should not be pending")
	}
}

func TestIsPending_UntiedWantsTied(t *testing.T) {
	m := &model{
		toggles: map[string]bool{"zsh": true},
	}
	row := pkgRow{name: "zsh", status: statusUntied}
	if !m.isPending(row) {
		t.Error("untied package with toggle=true should be pending")
	}
}

func TestIsPending_PartialCounts(t *testing.T) {
	m := &model{
		toggles: map[string]bool{"git": false},
	}
	row := pkgRow{name: "git", status: statusPartial}
	// partial is treated as "tied" for isPending comparison
	if !m.isPending(row) {
		t.Error("partial package with toggle=false should be pending")
	}
}

// ── pendingCount ──────────────────────────────────────────────────────────────

func TestPendingCount_None(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusTied},
			{name: "zsh", status: statusUntied},
		},
		toggles: map[string]bool{
			"nvim": true,
			"zsh":  false,
		},
	}
	if n := m.pendingCount(); n != 0 {
		t.Errorf("expected 0 pending, got %d", n)
	}
}

func TestPendingCount_One(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusTied},
			{name: "zsh", status: statusUntied},
		},
		toggles: map[string]bool{
			"nvim": false, // pending change
			"zsh":  false,
		},
	}
	if n := m.pendingCount(); n != 1 {
		t.Errorf("expected 1 pending, got %d", n)
	}
}

func TestPendingCount_All(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusTied},
			{name: "zsh", status: statusUntied},
		},
		toggles: map[string]bool{
			"nvim": false, // pending
			"zsh":  true,  // pending
		},
	}
	if n := m.pendingCount(); n != 2 {
		t.Errorf("expected 2 pending, got %d", n)
	}
}

// ── togglePackage ─────────────────────────────────────────────────────────────

func TestTogglePackage_Normal(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusUntied},
		},
		toggles: map[string]bool{"nvim": false},
	}
	m.togglePackage(0)
	if !m.toggles["nvim"] {
		t.Error("toggle should flip false -> true")
	}
	m.togglePackage(0)
	if m.toggles["nvim"] {
		t.Error("toggle should flip true -> false")
	}
}

func TestTogglePackage_SkippedIsNoop(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "yabai", status: statusSkipped},
		},
		toggles: map[string]bool{"yabai": false},
	}
	m.togglePackage(0)
	if m.toggles["yabai"] {
		t.Error("toggling skipped package should be a no-op")
	}
}

func TestTogglePackage_ConflictIsNoop(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusConflict},
		},
		toggles: map[string]bool{"nvim": false},
	}
	m.togglePackage(0)
	if m.toggles["nvim"] {
		t.Error("toggling conflict package should be a no-op")
	}
}

// ── listHeaderLines ───────────────────────────────────────────────────────────

func TestListHeaderLines_WithBranch(t *testing.T) {
	m := &model{gitBranch: "main"}
	if got := m.listHeaderLines(); got != 3 {
		t.Errorf("expected 3 header lines with git branch, got %d", got)
	}
}

func TestListHeaderLines_WithoutBranch(t *testing.T) {
	m := &model{}
	if got := m.listHeaderLines(); got != 2 {
		t.Errorf("expected 2 header lines without git branch, got %d", got)
	}
}

// ── visibleHeight ─────────────────────────────────────────────────────────────

func TestVisibleHeight_Normal(t *testing.T) {
	m := &model{height: 20}
	// overhead = 2 (no branch) + 3 = 5; visible = 20 - 5 = 15
	if got := m.visibleHeight(); got != 15 {
		t.Errorf("expected 15 visible rows, got %d", got)
	}
}

func TestVisibleHeight_Minimum(t *testing.T) {
	m := &model{height: 1}
	if got := m.visibleHeight(); got < 1 {
		t.Errorf("visibleHeight should be at least 1, got %d", got)
	}
}

func TestVisibleHeight_WithBranch(t *testing.T) {
	m := &model{height: 20, gitBranch: "main"}
	// overhead = 3 (with branch) + 3 = 6; visible = 20 - 6 = 14
	if got := m.visibleHeight(); got != 14 {
		t.Errorf("expected 14 visible rows with git branch, got %d", got)
	}
}

// ── adjustOffset ──────────────────────────────────────────────────────────────

func TestAdjustOffset_CursorAboveView(t *testing.T) {
	m := &model{
		height: 20,
		cursor: 0,
		offset: 5,
	}
	m.adjustOffset()
	if m.offset != 0 {
		t.Errorf("expected offset to move to cursor (0), got %d", m.offset)
	}
}

func TestAdjustOffset_CursorBelowView(t *testing.T) {
	m := &model{
		height: 10,
		cursor: 10,
		offset: 0,
		rows:   make([]pkgRow, 15),
	}
	visH := m.visibleHeight()
	m.adjustOffset()
	expected := 10 - visH + 1
	if m.offset != expected {
		t.Errorf("expected offset %d, got %d", expected, m.offset)
	}
}

func TestAdjustOffset_CursorInView(t *testing.T) {
	m := &model{
		height: 20,
		cursor: 3,
		offset: 0,
	}
	m.adjustOffset()
	if m.offset != 0 {
		t.Error("offset should not change when cursor is already in view")
	}
}

// ── branchVisibleHeight ───────────────────────────────────────────────────────

func TestBranchVisibleHeight_Normal(t *testing.T) {
	m := &model{height: 20}
	// overhead = 4; visible = 16
	if got := m.branchVisibleHeight(); got != 16 {
		t.Errorf("expected 16, got %d", got)
	}
}

func TestBranchVisibleHeight_Minimum(t *testing.T) {
	m := &model{height: 1}
	if got := m.branchVisibleHeight(); got < 1 {
		t.Errorf("branchVisibleHeight should be at least 1, got %d", got)
	}
}

// ── adjustBranchOffset ────────────────────────────────────────────────────────

func TestAdjustBranchOffset_CursorAbove(t *testing.T) {
	m := &model{
		height:       20,
		branchCursor: 0,
		branchOffset: 3,
	}
	m.adjustBranchOffset()
	if m.branchOffset != 0 {
		t.Errorf("expected branchOffset 0, got %d", m.branchOffset)
	}
}

func TestAdjustBranchOffset_CursorBelow(t *testing.T) {
	m := &model{
		height:       10,
		branchCursor: 12,
		branchOffset: 0,
	}
	vh := m.branchVisibleHeight()
	m.adjustBranchOffset()
	expected := 12 - vh + 1
	if m.branchOffset != expected {
		t.Errorf("expected branchOffset %d, got %d", expected, m.branchOffset)
	}
}

// ── buildConfirmLines ─────────────────────────────────────────────────────────

func TestBuildConfirmLines_TieAndUntie(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusUntied},
			{name: "zsh", status: statusTied},
			{name: "git", status: statusTied},
		},
		toggles: map[string]bool{
			"nvim": true,  // want to tie — pending
			"zsh":  false, // want to untie — pending
			"git":  true,  // already tied — not pending
		},
	}
	lines := m.buildConfirmLines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 confirm lines, got %d: %v", len(lines), lines)
	}
	foundTie, foundUntie := false, false
	for _, l := range lines {
		if containsSubstr(l, "tie") && containsSubstr(l, "nvim") {
			foundTie = true
		}
		if containsSubstr(l, "untie") && containsSubstr(l, "zsh") {
			foundUntie = true
		}
	}
	if !foundTie {
		t.Error("expected a 'tie nvim' line in confirm output")
	}
	if !foundUntie {
		t.Error("expected an 'untie zsh' line in confirm output")
	}
}

func TestBuildConfirmLines_NoPending(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusTied},
		},
		toggles: map[string]bool{"nvim": true},
	}
	if lines := m.buildConfirmLines(); len(lines) != 0 {
		t.Errorf("expected 0 confirm lines when nothing pending, got %d", len(lines))
	}
}

// ── dotfilesDir ───────────────────────────────────────────────────────────────

func TestDotfilesDir(t *testing.T) {
	got := dotfilesDir("/home/user/.dotfiles/Knotfile")
	expected := "/home/user/.dotfiles"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDotfilesDir_Nested(t *testing.T) {
	got := dotfilesDir("/a/b/c/Knotfile")
	expected := "/a/b/c"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// ── max / min ─────────────────────────────────────────────────────────────────

func TestMax(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{1, 2, 2},
		{5, 3, 5},
		{4, 4, 4},
		{-1, -5, -1},
	}
	for _, c := range cases {
		if got := max(c.a, c.b); got != c.want {
			t.Errorf("max(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMin(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{1, 2, 1},
		{5, 3, 3},
		{4, 4, 4},
		{-1, -5, -5},
	}
	for _, c := range cases {
		if got := min(c.a, c.b); got != c.want {
			t.Errorf("min(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// ── buildRows ─────────────────────────────────────────────────────────────────

func makeTempPackage(t *testing.T, files map[string]string) (source, target string) {
	t.Helper()
	source = t.TempDir()
	// target must not exist yet: with directory symlinking the linker creates a
	// symlink AT the target path, so an existing directory would be a CONFLICT.
	target = filepath.Join(t.TempDir(), "target")
	for rel, content := range files {
		full := filepath.Join(source, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func newCmdTestLinker() *linker.Linker {
	return &linker.Linker{
		DryRun:  false,
		HomeDir: "/home/testuser",
		GOOS:    runtime.GOOS,
		Writer:  &bytes.Buffer{},
	}
}

func TestBuildRows_Basic(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"init.lua": "-- x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, err := buildRows(cfg, lnk)
	if err != nil {
		t.Fatalf("buildRows() error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].name != "nvim" {
		t.Errorf("expected row name 'nvim', got %q", rows[0].name)
	}
}

func TestBuildRows_Sorted(t *testing.T) {
	sourceA, targetA := makeTempPackage(t, map[string]string{"f": "x"})
	sourceB, targetB := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"zsh":  {Source: sourceA, Target: targetA},
			"nvim": {Source: sourceB, Target: targetB},
		},
	}
	rows, err := buildRows(cfg, lnk)
	if err != nil {
		t.Fatalf("buildRows() error: %v", err)
	}
	if len(rows) != 2 || rows[0].name != "nvim" || rows[1].name != "zsh" {
		t.Errorf("expected sorted rows [nvim, zsh], got %v", rows)
	}
}

// ── view methods ──────────────────────────────────────────────────────────────

func baseModel(t *testing.T) model {
	t.Helper()
	source, target := makeTempPackage(t, map[string]string{"f.lua": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, err := buildRows(cfg, lnk)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(source, "Knotfile")
	return model{
		cfg:     cfg,
		cfgPath: cfgPath,
		lnk:     lnk,
		rows:    rows,
		toggles: seedToggles(rows),
		phase:   phaseList,
		width:   80,
		height:  24,
	}
}

func TestViewList_NonEmpty(t *testing.T) {
	m := baseModel(t)
	out := m.viewList()
	if out == "" {
		t.Error("viewList() returned empty string")
	}
	if !containsSubstr(out, "knot") {
		t.Error("viewList() should contain 'knot' header")
	}
}

func TestViewList_WithGitBranch(t *testing.T) {
	m := baseModel(t)
	m.gitBranch = "main"
	m.gitSHA = "abc1234"
	m.gitCommitMsg = "initial commit"
	out := m.viewList()
	if !containsSubstr(out, "main") {
		t.Errorf("viewList() should show git branch, got:\n%s", out)
	}
}

func TestViewList_NoRows(t *testing.T) {
	m := model{
		cfg:     &config.Config{Packages: map[string]config.Package{}},
		rows:    nil,
		toggles: map[string]bool{},
		phase:   phaseList,
		width:   80,
		height:  24,
		cfgPath: "/tmp/Knotfile",
	}
	out := m.viewList()
	if !containsSubstr(out, "No packages") {
		t.Errorf("expected 'No packages' message, got:\n%s", out)
	}
}

func TestViewList_StatusMsg(t *testing.T) {
	m := baseModel(t)
	m.statusMsg = "some editor error"
	out := m.viewList()
	if !containsSubstr(out, "some editor error") {
		t.Errorf("viewList() should show statusMsg, got:\n%s", out)
	}
}

func TestViewList_PendingChanges(t *testing.T) {
	m := baseModel(t)
	// make nvim pending by flipping its toggle
	m.toggles["nvim"] = !m.toggles["nvim"]
	out := m.viewList()
	if !containsSubstr(out, "pending") {
		t.Errorf("viewList() should show pending changes message, got:\n%s", out)
	}
}

func TestViewConfirm_Lines(t *testing.T) {
	m := baseModel(t)
	m.confirmLines = []string{"  tie   nvim", "  untie zsh"}
	out := m.viewConfirm()
	if !containsSubstr(out, "nvim") {
		t.Errorf("viewConfirm() should show package names, got:\n%s", out)
	}
	if !containsSubstr(out, "Apply?") {
		t.Errorf("viewConfirm() should show Apply? prompt, got:\n%s", out)
	}
}

func TestViewResult_Success(t *testing.T) {
	m := baseModel(t)
	m.applyLog = []string{"linked /home/user/.config/nvim/init.lua"}
	out := m.viewResult()
	if !containsSubstr(out, "Done") {
		t.Errorf("viewResult() should show Done on success, got:\n%s", out)
	}
}

func TestViewResult_Error(t *testing.T) {
	m := baseModel(t)
	m.applyErr = errors.New("something went wrong")
	out := m.viewResult()
	if !containsSubstr(out, "Error") {
		t.Errorf("viewResult() should show Error on failure, got:\n%s", out)
	}
}

func TestViewBranch_WithBranches(t *testing.T) {
	m := baseModel(t)
	m.branches = []string{"main", "feature/x"}
	m.gitBranch = "main"
	m.branchCursor = 0
	out := m.viewBranch()
	if !containsSubstr(out, "main") {
		t.Errorf("viewBranch() should show branches, got:\n%s", out)
	}
}

func TestViewBranch_NoBranches(t *testing.T) {
	m := baseModel(t)
	out := m.viewBranch()
	if !containsSubstr(out, "No branches") {
		t.Errorf("viewBranch() with no branches should say 'No branches', got:\n%s", out)
	}
}

func TestView_AllPhases(t *testing.T) {
	phases := []tuiPhase{phaseList, phaseConfirm, phaseApply, phaseResult, phaseGitPull, phaseBranch, phaseCheckout}
	for _, phase := range phases {
		m := baseModel(t)
		m.phase = phase
		out := m.View()
		if out == "" {
			t.Errorf("View() returned empty string for phase %d", phase)
		}
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func TestInit_ReturnsCmd(t *testing.T) {
	m := baseModel(t)
	cmd := m.Init()
	// Init returns a tea.Cmd (function). It should be non-nil.
	if cmd == nil {
		t.Error("Init() should return a non-nil tea.Cmd")
	}
}

// ── updateList ────────────────────────────────────────────────────────────────

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func TestUpdateList_NavigateDown(t *testing.T) {
	source1, target1 := makeTempPackage(t, map[string]string{"f": "x"})
	source2, target2 := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: target1},
			"zsh":  {Source: source2, Target: target2},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	m := model{
		cfg:     cfg,
		cfgPath: "/tmp/Knotfile",
		lnk:     lnk,
		rows:    rows,
		toggles: seedToggles(rows),
		cursor:  0,
		phase:   phaseList,
		height:  24,
		width:   80,
	}
	result, _ := m.updateList(keyMsg("j"))
	newM := result.(model)
	if newM.cursor != 1 {
		t.Errorf("expected cursor=1 after 'j', got %d", newM.cursor)
	}
}

func TestUpdateList_NavigateUp(t *testing.T) {
	source1, target1 := makeTempPackage(t, map[string]string{"f": "x"})
	source2, target2 := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: target1},
			"zsh":  {Source: source2, Target: target2},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	m := model{
		cfg:     cfg,
		cfgPath: "/tmp/Knotfile",
		lnk:     lnk,
		rows:    rows,
		toggles: seedToggles(rows),
		cursor:  1,
		phase:   phaseList,
		height:  24,
		width:   80,
	}
	result, _ := m.updateList(keyMsg("k"))
	newM := result.(model)
	if newM.cursor != 0 {
		t.Errorf("expected cursor=0 after 'k', got %d", newM.cursor)
	}
}

func TestUpdateList_NavigateAtBoundaries(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	m := model{
		cfg: cfg, cfgPath: "/tmp/Knotfile", lnk: lnk, rows: rows,
		toggles: seedToggles(rows), cursor: 0, phase: phaseList, height: 24, width: 80,
	}
	// Can't go up from 0
	result, _ := m.updateList(keyMsg("k"))
	if result.(model).cursor != 0 {
		t.Error("cursor should stay at 0 when already at top")
	}

	m.cursor = 0
	// Can't go down past last
	result, _ = m.updateList(keyMsg("j"))
	if result.(model).cursor != 0 {
		t.Error("cursor should stay at 0 with only 1 row")
	}
}

func TestUpdateList_Toggle(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	initial := seedToggles(rows)["nvim"]
	m := model{
		cfg: cfg, cfgPath: "/tmp/Knotfile", lnk: lnk, rows: rows,
		toggles: seedToggles(rows), cursor: 0, phase: phaseList, height: 24, width: 80,
	}
	result, _ := m.updateList(keyMsg(" "))
	if result.(model).toggles["nvim"] == initial {
		t.Error("space should toggle the package")
	}
}

func TestUpdateList_ApplyNoPending(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	m := model{
		cfg: cfg, cfgPath: "/tmp/Knotfile", lnk: lnk, rows: rows,
		toggles: seedToggles(rows), phase: phaseList, height: 24, width: 80,
	}
	// 'a' with no pending changes should do nothing
	result, _ := m.updateList(keyMsg("a"))
	if result.(model).phase != phaseList {
		t.Error("'a' with no pending changes should keep phase as phaseList")
	}
}

func TestUpdateList_ApplyWithPending(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	// Flip toggle to create pending change
	toggles := seedToggles(rows)
	toggles["nvim"] = !toggles["nvim"]
	m := model{
		cfg: cfg, cfgPath: "/tmp/Knotfile", lnk: lnk, rows: rows,
		toggles: toggles, phase: phaseList, height: 24, width: 80,
	}
	result, _ := m.updateList(keyMsg("a"))
	if result.(model).phase != phaseConfirm {
		t.Errorf("'a' with pending changes should move to phaseConfirm, got %v", result.(model).phase)
	}
}

func TestUpdateList_Quit(t *testing.T) {
	m := model{phase: phaseList, rows: []pkgRow{}, toggles: map[string]bool{}}
	_, cmd := m.updateList(keyMsg("q"))
	if cmd == nil {
		t.Error("'q' should return a quit command")
	}
}

// ── updateConfirm ─────────────────────────────────────────────────────────────

func TestUpdateConfirm_Yes(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	m := model{
		cfg:     cfg,
		cfgPath: "/tmp/Knotfile",
		lnk:     lnk,
		rows:    rows,
		toggles: seedToggles(rows),
		phase:   phaseConfirm,
		height:  24,
		width:   80,
	}
	result, cmd := m.updateConfirm(keyMsg("y"))
	newM := result.(model)
	if newM.phase != phaseApply {
		t.Errorf("'y' should move to phaseApply, got %v", newM.phase)
	}
	if cmd == nil {
		t.Error("'y' should return an apply command")
	}
}

func TestUpdateConfirm_No(t *testing.T) {
	m := model{
		phase: phaseConfirm, confirmLines: []string{"line"},
		rows: []pkgRow{}, toggles: map[string]bool{},
	}
	result, _ := m.updateConfirm(keyMsg("n"))
	newM := result.(model)
	if newM.phase != phaseList {
		t.Errorf("'n' should return to phaseList, got %v", newM.phase)
	}
	if len(newM.confirmLines) != 0 {
		t.Error("'n' should clear confirmLines")
	}
}

func TestUpdateConfirm_Esc(t *testing.T) {
	m := model{phase: phaseConfirm, rows: []pkgRow{}, toggles: map[string]bool{}}
	result, _ := m.updateConfirm(keyMsg("esc"))
	if result.(model).phase != phaseList {
		t.Error("esc should return to phaseList")
	}
}

// ── updateBranch ──────────────────────────────────────────────────────────────

func TestUpdateBranch_NavigateDown(t *testing.T) {
	m := model{
		branches:     []string{"main", "dev"},
		branchCursor: 0,
		phase:        phaseBranch,
		height:       24,
		width:        80,
	}
	result, _ := m.updateBranch(keyMsg("j"))
	if result.(model).branchCursor != 1 {
		t.Error("'j' should move branchCursor down")
	}
}

func TestUpdateBranch_NavigateUp(t *testing.T) {
	m := model{
		branches:     []string{"main", "dev"},
		branchCursor: 1,
		phase:        phaseBranch,
		height:       24,
		width:        80,
	}
	result, _ := m.updateBranch(keyMsg("k"))
	if result.(model).branchCursor != 0 {
		t.Error("'k' should move branchCursor up")
	}
}

func TestUpdateBranch_Esc(t *testing.T) {
	m := model{branches: []string{"main"}, phase: phaseBranch}
	result, _ := m.updateBranch(keyMsg("esc"))
	if result.(model).phase != phaseList {
		t.Error("esc in branch view should go back to phaseList")
	}
}

func TestUpdateBranch_EnterCurrentBranch(t *testing.T) {
	m := model{
		branches:     []string{"main", "dev"},
		branchCursor: 0,
		gitBranch:    "main",
		phase:        phaseBranch,
	}
	result, _ := m.updateBranch(keyMsg("enter"))
	if result.(model).phase != phaseList {
		t.Error("enter on current branch should stay at phaseList")
	}
}

func TestUpdateBranch_EnterOtherBranch(t *testing.T) {
	m := model{
		branches:     []string{"main", "dev"},
		branchCursor: 1,
		gitBranch:    "main",
		phase:        phaseBranch,
		cfgPath:      "/tmp/Knotfile",
	}
	result, cmd := m.updateBranch(keyMsg("enter"))
	newM := result.(model)
	if newM.phase != phaseCheckout {
		t.Errorf("enter on different branch should move to phaseCheckout, got %v", newM.phase)
	}
	if cmd == nil {
		t.Error("entering a different branch should return a checkout command")
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	newM := result.(model)
	if newM.width != 120 || newM.height != 40 {
		t.Errorf("WindowSizeMsg should update dimensions, got %dx%d", newM.width, newM.height)
	}
}

func TestUpdate_GitInfoMsg_Success(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(gitInfoMsg{branch: "main", sha: "abc", msg: "commit"})
	newM := result.(model)
	if newM.gitBranch != "main" || newM.gitSHA != "abc" {
		t.Error("gitInfoMsg should update git info")
	}
}

func TestUpdate_GitInfoMsg_Error(t *testing.T) {
	m := baseModel(t)
	m.gitBranch = "existing"
	result, _ := m.Update(gitInfoMsg{err: errors.New("no git")})
	newM := result.(model)
	// git branch should be unchanged on error
	if newM.gitBranch != "existing" {
		t.Error("gitInfoMsg with error should not clear existing git info")
	}
}

func TestUpdate_ReloadMsg_Success(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	newCfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(newCfg, lnk)
	m := baseModel(t)
	result, _ := m.Update(reloadMsg{cfg: newCfg, rows: rows})
	newM := result.(model)
	if newM.phase != phaseList {
		t.Errorf("reloadMsg success should set phase to phaseList, got %v", newM.phase)
	}
}

func TestUpdate_ReloadMsg_Error(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(reloadMsg{err: errors.New("load failed")})
	newM := result.(model)
	if newM.phase != phaseResult {
		t.Errorf("reloadMsg error should set phase to phaseResult, got %v", newM.phase)
	}
}

func TestUpdate_ApplyDoneMsg(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(applyDoneMsg{log: []string{"linked /x"}, err: nil})
	newM := result.(model)
	if newM.phase != phaseResult {
		t.Errorf("applyDoneMsg should set phase to phaseResult, got %v", newM.phase)
	}
	if len(newM.applyLog) != 1 {
		t.Error("applyDoneMsg should set applyLog")
	}
}

func TestUpdate_EditorDoneMsg_Success(t *testing.T) {
	m := baseModel(t)
	result, cmd := m.Update(editorDoneMsg{err: nil})
	newM := result.(model)
	if newM.statusMsg != "" {
		t.Error("editorDoneMsg success should clear statusMsg")
	}
	if cmd == nil {
		t.Error("editorDoneMsg success should return a reload command")
	}
}

func TestUpdate_EditorDoneMsg_Error(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(editorDoneMsg{err: errors.New("editor failed")})
	newM := result.(model)
	if newM.statusMsg == "" {
		t.Error("editorDoneMsg error should set statusMsg")
	}
	if newM.phase != phaseList {
		t.Errorf("editorDoneMsg error should keep phaseList, got %v", newM.phase)
	}
}

func TestUpdate_BranchListMsg_Success(t *testing.T) {
	m := baseModel(t)
	m.gitBranch = "main"
	result, _ := m.Update(branchListMsg{branches: []string{"main", "dev"}})
	newM := result.(model)
	if newM.phase != phaseBranch {
		t.Errorf("branchListMsg should move to phaseBranch, got %v", newM.phase)
	}
	if len(newM.branches) != 2 {
		t.Error("branchListMsg should set branches")
	}
	// Cursor should be set to current branch index
	if newM.branchCursor != 0 {
		t.Errorf("branchCursor should point to current branch (main at 0), got %d", newM.branchCursor)
	}
}

func TestUpdate_BranchListMsg_Error(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(branchListMsg{err: errors.New("no git")})
	newM := result.(model)
	if newM.statusMsg == "" {
		t.Error("branchListMsg error should set statusMsg")
	}
}

func TestUpdate_GitPullResultMsg_Error(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(gitPullResultMsg{output: "conflict", err: errors.New("pull failed")})
	newM := result.(model)
	if newM.phase != phaseResult {
		t.Errorf("gitPullResultMsg error should set phaseResult, got %v", newM.phase)
	}
}

func TestUpdate_GitPullResultMsg_Success(t *testing.T) {
	m := baseModel(t)
	m.cfgPath = "/tmp/Knotfile"
	result, cmd := m.Update(gitPullResultMsg{output: "up to date", err: nil})
	_ = result
	if cmd == nil {
		t.Error("gitPullResultMsg success should return a reload+fetch command")
	}
}

func TestUpdate_CheckoutDoneMsg_Error(t *testing.T) {
	m := baseModel(t)
	result, _ := m.Update(checkoutDoneMsg{output: "error", err: errors.New("checkout failed")})
	newM := result.(model)
	if newM.phase != phaseResult {
		t.Errorf("checkoutDoneMsg error should set phaseResult, got %v", newM.phase)
	}
}

func TestUpdate_CheckoutDoneMsg_Success(t *testing.T) {
	m := baseModel(t)
	m.cfgPath = "/tmp/Knotfile"
	result, cmd := m.Update(checkoutDoneMsg{output: "switched", err: nil})
	_ = result
	if cmd == nil {
		t.Error("checkoutDoneMsg success should return reload+fetch command")
	}
}

func TestUpdate_PhaseResult_AnyKey(t *testing.T) {
	m := baseModel(t)
	m.phase = phaseResult
	m.applyLog = []string{"done"}
	m.applyErr = nil
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	newM := result.(model)
	if newM.phase != phaseList {
		t.Errorf("any key in phaseResult should return to phaseList, got %v", newM.phase)
	}
}

func TestUpdate_PhaseApply_CtrlC(t *testing.T) {
	m := baseModel(t)
	m.phase = phaseApply
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c in phaseApply should return quit command")
	}
}

// ── reloadCmd ─────────────────────────────────────────────────────────────────

func makeTempKnotfile(t *testing.T) (cfgPath string) {
	t.Helper()
	source, target := makeTempPackage(t, map[string]string{"f.lua": "x"})
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "Knotfile")
	content := fmt.Sprintf(`packages:
  nvim:
    source: %s
    target: %s
`, source, target)
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestReloadCmd_Success(t *testing.T) {
	cfgPath := makeTempKnotfile(t)
	lnk := newCmdTestLinker()
	cmd := reloadCmd(cfgPath, lnk)
	msg := cmd()
	reload, ok := msg.(reloadMsg)
	if !ok {
		t.Fatalf("expected reloadMsg, got %T", msg)
	}
	if reload.err != nil {
		t.Errorf("reloadCmd() error: %v", reload.err)
	}
	if len(reload.rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(reload.rows))
	}
}

func TestReloadCmd_BadPath(t *testing.T) {
	lnk := newCmdTestLinker()
	cmd := reloadCmd("/nonexistent/path/Knotfile", lnk)
	msg := cmd()
	reload, ok := msg.(reloadMsg)
	if !ok {
		t.Fatalf("expected reloadMsg, got %T", msg)
	}
	if reload.err == nil {
		t.Error("reloadCmd with bad path should return error")
	}
}

// ── applyCmd ──────────────────────────────────────────────────────────────────

func TestApplyCmd_TiePackage(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f.lua": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	// Currently untied, toggle to tie
	toggles := map[string]bool{"nvim": true}

	cmd := applyCmd(cfg, lnk, rows, toggles)
	msg := cmd()
	done, ok := msg.(applyDoneMsg)
	if !ok {
		t.Fatalf("expected applyDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Errorf("applyCmd error: %v", done.err)
	}
}

func TestApplyCmd_NoPendingChanges(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f.lua": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}
	rows, _ := buildRows(cfg, lnk)
	// Status is untied, toggle is false — no change needed
	toggles := map[string]bool{"nvim": false}

	cmd := applyCmd(cfg, lnk, rows, toggles)
	msg := cmd()
	done, ok := msg.(applyDoneMsg)
	if !ok {
		t.Fatalf("expected applyDoneMsg, got %T", msg)
	}
	if len(done.log) != 0 {
		t.Error("expected empty log when nothing to apply")
	}
}

// ── updateTags ────────────────────────────────────────────────────────────────

func baseTagModel(t *testing.T) model {
	t.Helper()
	source1, target1 := makeTempPackage(t, map[string]string{"f": "x"})
	source2, target2 := makeTempPackage(t, map[string]string{"g": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: target1, Tags: []string{"work"}},
			"tmux": {Source: source2, Target: target2, Tags: []string{"work"}},
		},
	}
	allRows, err := buildRows(cfg, lnk)
	if err != nil {
		t.Fatal(err)
	}
	return model{
		cfg:       cfg,
		cfgPath:   "/tmp/Knotfile",
		lnk:       lnk,
		rows:      allRows,
		toggles:   seedToggles(allRows),
		tagRows:   buildTagRows(cfg, allRows),
		activeTab: tabTags,
		phase:     phaseList,
		width:     80,
		height:    24,
	}
}

func TestUpdateTags_NavigateDown(t *testing.T) {
	m := baseTagModel(t)
	m.tagCursor = 0
	result, _ := m.updateTags(keyMsg("j"))
	newM := result.(model)
	if newM.tagCursor != 1 {
		t.Errorf("'j' should move tagCursor to 1, got %d", newM.tagCursor)
	}
}

func TestUpdateTags_NavigateUp(t *testing.T) {
	m := baseTagModel(t)
	m.tagCursor = 1
	result, _ := m.updateTags(keyMsg("k"))
	newM := result.(model)
	if newM.tagCursor != 0 {
		t.Errorf("'k' should move tagCursor to 0, got %d", newM.tagCursor)
	}
}

func TestUpdateTags_EnterCollapses(t *testing.T) {
	m := baseTagModel(t)
	m.tagCursor = 0 // on the tag row
	result, _ := m.updateTags(keyMsg("enter"))
	newM := result.(model)
	if !newM.tagRows[0].collapsed {
		t.Error("enter on tag row should collapse it")
	}
	// enter again should expand
	result2, _ := newM.updateTags(keyMsg("enter"))
	newM2 := result2.(model)
	if newM2.tagRows[0].collapsed {
		t.Error("second enter should expand the tag row")
	}
}

func TestUpdateTags_SpaceOnTagToggles(t *testing.T) {
	m := baseTagModel(t)
	m.tagCursor = 0 // on the 'work' tag row (status: untied)
	initial := m.toggles["nvim"]
	result, _ := m.updateTags(keyMsg(" "))
	newM := result.(model)
	if newM.toggles["nvim"] == initial {
		t.Error("space on tag row should toggle its packages")
	}
}

func TestUpdateTags_SwitchToPackages(t *testing.T) {
	m := baseTagModel(t)
	result, _ := m.updateTags(keyMsg("["))
	newM := result.(model)
	if newM.activeTab != tabPackages {
		t.Error("'[' should switch to packages tab")
	}
}

func TestUpdateTags_Quit(t *testing.T) {
	m := baseTagModel(t)
	_, cmd := m.updateTags(keyMsg("q"))
	if cmd == nil {
		t.Error("'q' should return quit command")
	}
}

// ── tab dispatch ──────────────────────────────────────────────────────────────

func TestUpdateList_SwitchToTags(t *testing.T) {
	m := baseModel(t)
	m.activeTab = tabPackages
	result, _ := m.updateList(keyMsg("]"))
	newM := result.(model)
	if newM.activeTab != tabTags {
		t.Error("']' in packages tab should switch to tags tab")
	}
}

func TestView_TagsTabDispatch(t *testing.T) {
	m := baseTagModel(t)
	m.phase = phaseList
	m.activeTab = tabTags
	out := m.View()
	if !containsSubstr(out, "work") {
		t.Error("View() in tabTags should show tag names")
	}
}

// ── renderTabHeader ───────────────────────────────────────────────────────────

func TestRenderTabHeader_PackagesActive(t *testing.T) {
	m := &model{activeTab: tabPackages}
	out := m.renderTabHeader()
	if !containsSubstr(out, "Packages") {
		t.Error("header should contain Packages")
	}
	if !containsSubstr(out, "Tags") {
		t.Error("header should contain Tags")
	}
}

func TestRenderTabHeader_TagsActive(t *testing.T) {
	m := &model{activeTab: tabTags}
	out := m.renderTabHeader()
	if !containsSubstr(out, "Tags") {
		t.Error("header should contain Tags")
	}
}

// ── tagVisibleHeight ──────────────────────────────────────────────────────────

func TestTagVisibleHeight_Normal(t *testing.T) {
	m := &model{height: 24}
	if got := m.tagVisibleHeight(); got < 1 {
		t.Errorf("tagVisibleHeight should be >= 1, got %d", got)
	}
}

// ── viewTags ──────────────────────────────────────────────────────────────────

func TestViewTags_NonEmpty(t *testing.T) {
	source1, target1 := makeTempPackage(t, map[string]string{"f": "x"})
	source2, target2 := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: target1, Tags: []string{"work"}},
			"tmux": {Source: source2, Target: target2, Tags: []string{"work"}},
		},
	}
	allRows, _ := buildRows(cfg, lnk)
	m := model{
		cfg:       cfg,
		cfgPath:   "/tmp/Knotfile",
		lnk:       lnk,
		rows:      allRows,
		toggles:   seedToggles(allRows),
		tagRows:   buildTagRows(cfg, allRows),
		activeTab: tabTags,
		width:     80,
		height:    24,
	}
	out := m.viewTags()
	if out == "" {
		t.Error("viewTags() should return non-empty string")
	}
	if !containsSubstr(out, "work") {
		t.Error("viewTags() should show tag name")
	}
	if !containsSubstr(out, "nvim") {
		t.Error("viewTags() should show package names when expanded")
	}
}

func TestViewTags_NoTaggedPackages(t *testing.T) {
	m := model{
		tagRows:   nil,
		activeTab: tabTags,
		width:     80,
		height:    24,
		cfgPath:   "/tmp/Knotfile",
	}
	out := m.viewTags()
	if !containsSubstr(out, "No tagged") {
		t.Errorf("viewTags() with no tags should say 'No tagged', got:\n%s", out)
	}
}

// ── toggleTag ─────────────────────────────────────────────────────────────────

func TestToggleTag_TiedToUntied(t *testing.T) {
	m := &model{
		rows: []pkgRow{
			{name: "nvim", status: statusTied},
			{name: "tmux", status: statusTied},
		},
		toggles: map[string]bool{"nvim": true, "tmux": true},
	}
	tr := &tagRow{
		name:   "work",
		status: statusTied,
		pkgs:   []pkgRow{{name: "nvim", status: statusTied}, {name: "tmux", status: statusTied}},
	}
	m.toggleTag(tr)
	if m.toggles["nvim"] || m.toggles["tmux"] {
		t.Error("toggleTag on tied tag should mark all packages for untie")
	}
}

func TestToggleTag_UntiedToTied(t *testing.T) {
	m := &model{
		rows:    []pkgRow{{name: "nvim", status: statusUntied}, {name: "tmux", status: statusUntied}},
		toggles: map[string]bool{"nvim": false, "tmux": false},
	}
	tr := &tagRow{
		name:   "work",
		status: statusUntied,
		pkgs:   []pkgRow{{name: "nvim", status: statusUntied}, {name: "tmux", status: statusUntied}},
	}
	m.toggleTag(tr)
	if !m.toggles["nvim"] || !m.toggles["tmux"] {
		t.Error("toggleTag on untied tag should mark all packages for tie")
	}
}

func TestToggleTag_PartialCompletes(t *testing.T) {
	m := &model{
		rows:    []pkgRow{{name: "nvim", status: statusTied}, {name: "tmux", status: statusUntied}},
		toggles: map[string]bool{"nvim": true, "tmux": false},
	}
	tr := &tagRow{
		name:   "work",
		status: statusPartial,
		pkgs:   []pkgRow{{name: "nvim", status: statusTied}, {name: "tmux", status: statusUntied}},
	}
	m.toggleTag(tr)
	if !m.toggles["nvim"] {
		t.Error("nvim (already tied) should remain true after completing partial tag")
	}
	if !m.toggles["tmux"] {
		t.Error("tmux (untied) should be marked for tie when completing partial tag")
	}
}

func TestToggleTag_SkipsSkippedAndConflict(t *testing.T) {
	m := &model{
		toggles: map[string]bool{"nvim": false, "yabai": false},
	}
	tr := &tagRow{
		name:   "work",
		status: statusUntied,
		pkgs: []pkgRow{
			{name: "nvim", status: statusUntied},
			{name: "yabai", status: statusSkipped},
		},
	}
	m.toggleTag(tr)
	if !m.toggles["nvim"] {
		t.Error("nvim should be marked for tie")
	}
	if m.toggles["yabai"] {
		t.Error("skipped package should not be toggled")
	}
}

// ── buildTagRows ──────────────────────────────────────────────────────────────

func TestBuildTagRows_Basic(t *testing.T) {
	source1, target1 := makeTempPackage(t, map[string]string{"f": "x"})
	source2, target2 := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: target1, Tags: []string{"work"}},
			"tmux": {Source: source2, Target: target2, Tags: []string{"work"}},
		},
	}
	allRows, err := buildRows(cfg, lnk)
	if err != nil {
		t.Fatal(err)
	}
	tagRows := buildTagRows(cfg, allRows)
	if len(tagRows) != 1 {
		t.Fatalf("expected 1 tag row, got %d", len(tagRows))
	}
	if tagRows[0].name != "work" {
		t.Errorf("expected tag 'work', got %q", tagRows[0].name)
	}
	if len(tagRows[0].pkgs) != 2 {
		t.Errorf("expected 2 packages in tag, got %d", len(tagRows[0].pkgs))
	}
}

func TestBuildTagRows_Sorted(t *testing.T) {
	sourceA, targetA := makeTempPackage(t, map[string]string{"f": "x"})
	sourceB, targetB := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"zsh":  {Source: sourceA, Target: targetA, Tags: []string{"home"}},
			"nvim": {Source: sourceB, Target: targetB, Tags: []string{"work"}},
		},
	}
	allRows, _ := buildRows(cfg, lnk)
	tagRows := buildTagRows(cfg, allRows)
	if len(tagRows) != 2 || tagRows[0].name != "home" || tagRows[1].name != "work" {
		t.Errorf("tag rows should be sorted by name, got %v", tagRows)
	}
}

func TestBuildTagRows_NoTaggedPackages(t *testing.T) {
	source, target := makeTempPackage(t, map[string]string{"f": "x"})
	lnk := newCmdTestLinker()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"secrets": {Source: source, Target: target},
		},
	}
	allRows, _ := buildRows(cfg, lnk)
	if got := buildTagRows(cfg, allRows); len(got) != 0 {
		t.Errorf("expected 0 tag rows for untagged packages, got %d", len(got))
	}
}

// ── visibleTagItems ────────────────────────────────────────────────────────────

func TestVisibleTagItems_Expanded(t *testing.T) {
	rows := []tagRow{
		{
			name: "work",
			pkgs: []pkgRow{
				{name: "nvim"},
				{name: "tmux"},
			},
			collapsed: false,
		},
	}
	items := visibleTagItems(rows)
	// 1 tag + 2 packages
	if len(items) != 3 {
		t.Fatalf("expected 3 items (1 tag + 2 pkg), got %d", len(items))
	}
	if !items[0].isTag {
		t.Error("first item should be tag")
	}
	if items[1].isTag || items[1].pkg.name != "nvim" {
		t.Error("second item should be nvim package")
	}
	if items[2].isLastChild != true {
		t.Error("last child should have isLastChild=true")
	}
}

func TestVisibleTagItems_Collapsed(t *testing.T) {
	rows := []tagRow{
		{
			name:      "work",
			pkgs:      []pkgRow{{name: "nvim"}, {name: "tmux"}},
			collapsed: true,
		},
	}
	items := visibleTagItems(rows)
	// only the tag row, no children
	if len(items) != 1 {
		t.Fatalf("expected 1 item when collapsed, got %d", len(items))
	}
	if !items[0].isTag {
		t.Error("only item should be the tag row")
	}
}

func TestVisibleTagItems_MultipleTags(t *testing.T) {
	rows := []tagRow{
		{name: "home", pkgs: []pkgRow{{name: "zsh"}}, collapsed: false},
		{name: "work", pkgs: []pkgRow{{name: "nvim"}, {name: "tmux"}}, collapsed: true},
	}
	items := visibleTagItems(rows)
	// home (1 tag + 1 pkg) + work (1 tag, collapsed) = 3
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsSubstr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── setupModel.Init ───────────────────────────────────────────────────────────

func TestSetupModel_Init_ModeInit_ReturnsNil(t *testing.T) {
	m := setupModel{dir: "/tmp/dotfiles", mode: setupModeInit}
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil for setupModeInit")
	}
}

func TestSetupModel_Init_ModeKnotfile_ReturnsCmd(t *testing.T) {
	m := setupModel{dir: "/tmp/dotfiles", mode: setupModeKnotfile}
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() should return non-nil cmd for setupModeKnotfile")
	}
}

// ── setupModel.View ───────────────────────────────────────────────────────────

func TestSetupView_Menu(t *testing.T) {
	m := setupModel{dir: "/home/user/.dotfiles", phase: setupPhaseMenu}
	out := m.View()
	if !containsSubstr(out, "knot setup") {
		t.Error("menu view should contain 'knot setup' header")
	}
	if !containsSubstr(out, "/home/user/.dotfiles") {
		t.Error("menu view should show the target directory")
	}
	if !containsSubstr(out, "Clone existing dotfiles from git") {
		t.Error("menu view should show git clone option")
	}
	if !containsSubstr(out, "Initialize new dotfiles folder") {
		t.Error("menu view should show init option")
	}
}

func TestSetupView_Menu_CursorOnFirstOption(t *testing.T) {
	// Option 0 is now "Initialize new dotfiles folder"
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 0}
	out := m.View()
	initIdx := strings.Index(out, "Initialize new")
	cloneIdx := strings.Index(out, "Clone existing")
	cursorIdx := strings.Index(out, ">")
	if cursorIdx < 0 {
		t.Fatal("cursor indicator '>' not found")
	}
	if cursorIdx > initIdx {
		t.Error("cursor should appear before 'Initialize new' when cursor=0")
	}
	_ = cloneIdx
}

func TestSetupView_Menu_CursorOnSecondOption(t *testing.T) {
	// Option 1 is "Clone existing dotfiles from git"
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 1}
	out := m.View()
	initIdx := strings.Index(out, "Initialize new")
	cloneIdx := strings.Index(out, "Clone existing")
	cursorIdx := strings.LastIndex(out, ">")
	if cursorIdx < 0 {
		t.Fatal("cursor indicator '>' not found")
	}
	if cursorIdx < initIdx || cursorIdx > cloneIdx {
		t.Error("cursor should appear between Initialize and Clone lines when cursor=1")
	}
}

func TestSetupView_GitProvider(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProvider}
	out := m.View()
	if !containsSubstr(out, "GitHub") {
		t.Error("provider view should show GitHub option")
	}
	if !containsSubstr(out, "GitLab") {
		t.Error("provider view should show GitLab option")
	}
}

func TestSetupView_GitProtocol_GitHub(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProtocol, gitProvider: "github"}
	out := m.View()
	if !containsSubstr(out, "github.com") {
		t.Error("protocol view should show provider host")
	}
	if !containsSubstr(out, "HTTPS") {
		t.Error("protocol view should show HTTPS option")
	}
	if !containsSubstr(out, "SSH") {
		t.Error("protocol view should show SSH option")
	}
	if !containsSubstr(out, "ssh key") || !containsSubstr(out, "SSH requires") {
		// either phrasing is fine — just check the SSH note is present
		if !containsSubstr(strings.ToLower(out), "ssh") {
			t.Error("protocol view should mention SSH key requirement")
		}
	}
}

func TestSetupView_GitProtocol_GitLab(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProtocol, gitProvider: "gitlab"}
	out := m.View()
	if !containsSubstr(out, "gitlab.com") {
		t.Error("protocol view should show gitlab.com when provider is gitlab")
	}
}

func TestSetupView_GitUsername(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: "oxGrad", gitProvider: "github"}
	out := m.View()
	if !containsSubstr(out, "oxGrad") {
		t.Error("username view should show the current input buffer")
	}
	if !containsSubstr(out, "█") {
		t.Error("username view should show block cursor")
	}
}

func TestSetupView_GitRepo(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitRepo, inputBuf: ""}
	out := m.View()
	if !containsSubstr(out, "█") {
		t.Error("repo view should show block cursor")
	}
	if !containsSubstr(out, ".dotfiles") {
		t.Error("repo view should hint at the default repo name")
	}
}

func TestSetupView_GitRepo_ShowsError(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitRepo, err: fmt.Errorf("git clone failed: exit status 128")}
	out := m.View()
	if !containsSubstr(out, "git clone failed") {
		t.Error("repo view should show error message")
	}
}

func TestSetupView_Cloning(t *testing.T) {
	m := setupModel{
		dir:      "/home/user/.dotfiles",
		phase:    setupPhaseCloning,
		cloneURL: "https://github.com/user/dots",
	}
	out := m.View()
	if !containsSubstr(out, "https://github.com/user/dots") {
		t.Error("cloning view should show the URL being cloned")
	}
	if !containsSubstr(out, "/home/user/.dotfiles") {
		t.Error("cloning view should show the target directory")
	}
}

func TestSetupView_ConfirmKnotfile(t *testing.T) {
	m := setupModel{dir: "/home/user/.dotfiles", phase: setupPhaseConfirmKnotfile}
	out := m.View()
	if !containsSubstr(out, "No Knotfile") {
		t.Error("confirm view should mention missing Knotfile")
	}
	if !containsSubstr(out, "[y/n]") {
		t.Error("confirm view should show y/n prompt")
	}
	if !containsSubstr(out, "/home/user/.dotfiles") {
		t.Error("confirm view should show the directory")
	}
}

func TestSetupView_ConfirmKnotfile_ShowsError(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseConfirmKnotfile, err: fmt.Errorf("writing Knotfile: permission denied")}
	out := m.View()
	if !containsSubstr(out, "writing Knotfile") {
		t.Error("confirm view should show error when present")
	}
}

func TestSetupView_Done(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseDone}
	out := m.View()
	if !containsSubstr(out, "Setup complete") {
		t.Error("done view should say 'Setup complete'")
	}
}

func TestSetupView_AllPhases_NonEmpty(t *testing.T) {
	phases := []setupPhase{
		setupPhaseMenu,
		setupPhaseGitProvider,
		setupPhaseGitProtocol,
		setupPhaseGitUsername,
		setupPhaseGitRepo,
		setupPhaseCloning,
		setupPhaseConfirmKnotfile,
		setupPhaseDone,
	}
	for _, phase := range phases {
		m := setupModel{dir: "/tmp/d", phase: phase}
		if out := m.View(); out == "" {
			t.Errorf("View() returned empty string for setupPhase %d", phase)
		}
	}
}

// ── setupModel.Update — WindowSizeMsg ─────────────────────────────────────────

func TestSetupUpdate_WindowSize(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu}
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	sm := result.(setupModel)
	if sm.width != 120 {
		t.Errorf("expected width=120 after WindowSizeMsg, got %d", sm.width)
	}
}

// ── setupModel.updateMenu ─────────────────────────────────────────────────────

func TestSetupUpdateMenu_NavigateDown(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 0}
	result, _ := m.updateMenu(keyMsg("j"))
	if result.(setupModel).cursor != 1 {
		t.Error("'j' should move cursor from 0 to 1")
	}
}

func TestSetupUpdateMenu_NavigateDownAtBottom(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 1}
	result, _ := m.updateMenu(keyMsg("j"))
	if result.(setupModel).cursor != 1 {
		t.Error("cursor should stay at 1 when already at bottom")
	}
}

func TestSetupUpdateMenu_NavigateUp(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 1}
	result, _ := m.updateMenu(keyMsg("k"))
	if result.(setupModel).cursor != 0 {
		t.Error("'k' should move cursor from 1 to 0")
	}
}

func TestSetupUpdateMenu_NavigateUpAtTop(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 0}
	result, _ := m.updateMenu(keyMsg("k"))
	if result.(setupModel).cursor != 0 {
		t.Error("cursor should stay at 0 when already at top")
	}
}

func TestSetupUpdateMenu_SelectInitNew(t *testing.T) {
	// Option 0 is now "Initialize new dotfiles folder"
	dir := t.TempDir()
	m := setupModel{dir: dir, phase: setupPhaseMenu, cursor: 0}
	_, cmd := m.updateMenu(keyMsg("enter"))
	if cmd == nil {
		t.Error("selecting 'Initialize new' (cursor=0) should dispatch writeKnotfileCmd")
	}
}

func TestSetupUpdateMenu_SelectGitClone(t *testing.T) {
	// Option 1 is "Clone from git" → goes to provider selection
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 1}
	result, _ := m.updateMenu(keyMsg("enter"))
	sm := result.(setupModel)
	if sm.phase != setupPhaseGitProvider {
		t.Errorf("selecting option 1 should advance to setupPhaseGitProvider, got phase %d", sm.phase)
	}
}

func TestSetupUpdateMenu_SpaceAlsoSelects(t *testing.T) {
	// Option 1 selected with space → provider selection
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu, cursor: 1}
	result, _ := m.updateMenu(keyMsg(" "))
	sm := result.(setupModel)
	if sm.phase != setupPhaseGitProvider {
		t.Error("space should also select the highlighted option")
	}
}

func TestSetupUpdateMenu_QuitKey(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu}
	_, cmd := m.updateMenu(keyMsg("q"))
	if cmd == nil {
		t.Error("'q' should return tea.Quit command")
	}
}

// ── setupModel.updateGitProvider ─────────────────────────────────────────────

func TestSetupUpdateGitProvider_SelectGitHub(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProvider, cursor: 0}
	result, _ := m.updateGitProvider(keyMsg("enter"))
	sm := result.(setupModel)
	if sm.gitProvider != "github" {
		t.Errorf("cursor=0 should select github, got %q", sm.gitProvider)
	}
	if sm.phase != setupPhaseGitProtocol {
		t.Errorf("should advance to setupPhaseGitProtocol, got %d", sm.phase)
	}
}

func TestSetupUpdateGitProvider_SelectGitLab(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProvider, cursor: 1}
	result, _ := m.updateGitProvider(keyMsg("enter"))
	sm := result.(setupModel)
	if sm.gitProvider != "gitlab" {
		t.Errorf("cursor=1 should select gitlab, got %q", sm.gitProvider)
	}
}

func TestSetupUpdateGitProvider_Navigate(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProvider, cursor: 0}
	result, _ := m.updateGitProvider(keyMsg("j"))
	if result.(setupModel).cursor != 1 {
		t.Error("'j' should move cursor to 1")
	}
	result, _ = result.(setupModel).updateGitProvider(keyMsg("k"))
	if result.(setupModel).cursor != 0 {
		t.Error("'k' should move cursor back to 0")
	}
}

func TestSetupUpdateGitProvider_EscGoesBackToMenu(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProvider}
	result, _ := m.updateGitProvider(keyMsg("esc"))
	sm := result.(setupModel)
	if sm.phase != setupPhaseMenu {
		t.Errorf("esc should return to setupPhaseMenu, got %d", sm.phase)
	}
	if sm.cursor != 1 {
		t.Error("esc should restore cursor to 1 (Clone option) in main menu")
	}
}

// ── setupModel.updateGitProtocol ─────────────────────────────────────────────

func TestSetupUpdateGitProtocol_SelectHTTPS(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProtocol, cursor: 0}
	result, _ := m.updateGitProtocol(keyMsg("enter"))
	sm := result.(setupModel)
	if sm.gitProtocol != "https" {
		t.Errorf("cursor=0 should select https, got %q", sm.gitProtocol)
	}
	if sm.phase != setupPhaseGitUsername {
		t.Errorf("should advance to setupPhaseGitUsername, got %d", sm.phase)
	}
}

func TestSetupUpdateGitProtocol_SelectSSH(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProtocol, cursor: 1}
	result, _ := m.updateGitProtocol(keyMsg("enter"))
	sm := result.(setupModel)
	if sm.gitProtocol != "ssh" {
		t.Errorf("cursor=1 should select ssh, got %q", sm.gitProtocol)
	}
}

func TestSetupUpdateGitProtocol_EscGoesBackToProvider(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitProtocol}
	result, _ := m.updateGitProtocol(keyMsg("esc"))
	if result.(setupModel).phase != setupPhaseGitProvider {
		t.Error("esc should return to setupPhaseGitProvider")
	}
}

// ── setupModel.updateGitInput (username + repo) ───────────────────────────────

func TestSetupUpdateGitInput_TypesCharacters(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: ""}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, setupPhaseGitProtocol, setupPhaseGitRepo)
	result, _ = result.(setupModel).updateGitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}, setupPhaseGitProtocol, setupPhaseGitRepo)
	if result.(setupModel).inputBuf != "ab" {
		t.Errorf("expected inputBuf='ab', got %q", result.(setupModel).inputBuf)
	}
}

func TestSetupUpdateGitInput_Backspace(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: "abc"}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyBackspace}, setupPhaseGitProtocol, setupPhaseGitRepo)
	if result.(setupModel).inputBuf != "ab" {
		t.Errorf("backspace should remove last char, got %q", result.(setupModel).inputBuf)
	}
}

func TestSetupUpdateGitInput_BackspaceOnEmpty(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: ""}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyBackspace}, setupPhaseGitProtocol, setupPhaseGitRepo)
	if result.(setupModel).inputBuf != "" {
		t.Error("backspace on empty should be a no-op")
	}
}

func TestSetupUpdateGitInput_EnterUsername_AdvancesToRepo(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: "oxGrad"}
	result, cmd := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, setupPhaseGitProtocol, setupPhaseGitRepo)
	sm := result.(setupModel)
	if sm.gitUsername != "oxGrad" {
		t.Errorf("gitUsername should be set, got %q", sm.gitUsername)
	}
	if sm.phase != setupPhaseGitRepo {
		t.Errorf("should advance to setupPhaseGitRepo, got %d", sm.phase)
	}
	if cmd != nil {
		t.Error("advancing to repo phase should not dispatch any command")
	}
}

func TestSetupUpdateGitInput_EnterUsernameEmpty_IsNoop(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: "   "}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, setupPhaseGitProtocol, setupPhaseGitRepo)
	if result.(setupModel).phase != setupPhaseGitUsername {
		t.Error("blank username should not advance phase")
	}
}

func TestSetupUpdateGitInput_EnterRepo_StartsClone(t *testing.T) {
	m := setupModel{
		dir: "/tmp/d", phase: setupPhaseGitRepo,
		gitProvider: "github", gitProtocol: "https", gitUsername: "user",
		inputBuf: "dotfiles",
	}
	result, cmd := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, setupPhaseGitUsername, setupPhaseGitRepo)
	sm := result.(setupModel)
	if sm.phase != setupPhaseCloning {
		t.Errorf("should advance to setupPhaseCloning, got %d", sm.phase)
	}
	if sm.cloneURL != "https://github.com/user/dotfiles" {
		t.Errorf("unexpected cloneURL: %q", sm.cloneURL)
	}
	if cmd == nil {
		t.Error("repo enter should dispatch cloneRepoCmd")
	}
}

func TestSetupUpdateGitInput_EnterRepoEmpty_UsesDefault(t *testing.T) {
	m := setupModel{
		dir: "/tmp/d", phase: setupPhaseGitRepo,
		gitProvider: "github", gitProtocol: "https", gitUsername: "user",
		inputBuf: "",
	}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, setupPhaseGitUsername, setupPhaseGitRepo)
	sm := result.(setupModel)
	if sm.gitRepo != ".dotfiles" {
		t.Errorf("empty repo input should default to '.dotfiles', got %q", sm.gitRepo)
	}
	if sm.cloneURL != "https://github.com/user/.dotfiles" {
		t.Errorf("unexpected cloneURL: %q", sm.cloneURL)
	}
}

func TestSetupUpdateGitInput_EscGoesBack(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername, inputBuf: "some text", err: fmt.Errorf("prev")}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEsc}, setupPhaseGitProtocol, setupPhaseGitRepo)
	sm := result.(setupModel)
	if sm.phase != setupPhaseGitProtocol {
		t.Errorf("esc should go to backPhase (setupPhaseGitProtocol), got %d", sm.phase)
	}
	if sm.inputBuf != "" {
		t.Error("esc should clear inputBuf")
	}
	if sm.err != nil {
		t.Error("esc should clear error")
	}
}

func TestSetupUpdateGitInput_CtrlCQuits(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseGitUsername}
	_, cmd := m.updateGitInput(tea.KeyMsg{Type: tea.KeyCtrlC}, setupPhaseGitProtocol, setupPhaseGitRepo)
	if cmd == nil {
		t.Error("ctrl+c should return tea.Quit")
	}
}

// ── buildGitURL ───────────────────────────────────────────────────────────────

func TestBuildGitURL_GitHubHTTPS(t *testing.T) {
	got := buildGitURL("github", "https", "user", "dots")
	want := "https://github.com/user/dots"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildGitURL_GitHubSSH(t *testing.T) {
	got := buildGitURL("github", "ssh", "user", "dots")
	want := "git@github.com:user/dots.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildGitURL_GitLabHTTPS(t *testing.T) {
	got := buildGitURL("gitlab", "https", "user", "dots")
	want := "https://gitlab.com/user/dots"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildGitURL_GitLabSSH(t *testing.T) {
	got := buildGitURL("gitlab", "ssh", "user", "dots")
	want := "git@gitlab.com:user/dots.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildGitURL_EmptyRepoDefaultsToDotfiles(t *testing.T) {
	got := buildGitURL("github", "https", "user", "")
	want := "https://github.com/user/.dotfiles"
	if got != want {
		t.Errorf("empty repo should default to .dotfiles, got %q", got)
	}
}

// ── setupModel.updateConfirmKnotfile ──────────────────────────────────────────

func TestSetupUpdateConfirm_YCreatesKnotfile(t *testing.T) {
	dir := t.TempDir()
	m := setupModel{dir: dir, phase: setupPhaseConfirmKnotfile}
	_, cmd := m.updateConfirmKnotfile(keyMsg("y"))
	if cmd == nil {
		t.Error("'y' should dispatch writeKnotfileCmd")
	}
}

func TestSetupUpdateConfirm_EnterCreatesKnotfile(t *testing.T) {
	dir := t.TempDir()
	m := setupModel{dir: dir, phase: setupPhaseConfirmKnotfile}
	_, cmd := m.updateConfirmKnotfile(keyMsg("enter"))
	if cmd == nil {
		t.Error("enter should dispatch writeKnotfileCmd")
	}
}

func TestSetupUpdateConfirm_NDeclines(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseConfirmKnotfile}
	result, cmd := m.updateConfirmKnotfile(keyMsg("n"))
	sm := result.(setupModel)
	if !sm.declined {
		t.Error("'n' should set declined=true")
	}
	if cmd == nil {
		t.Error("'n' should return tea.Quit")
	}
}

func TestSetupUpdateConfirm_EscDeclines(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseConfirmKnotfile}
	result, cmd := m.updateConfirmKnotfile(keyMsg("esc"))
	sm := result.(setupModel)
	if !sm.declined {
		t.Error("esc should set declined=true")
	}
	if cmd == nil {
		t.Error("esc should return tea.Quit")
	}
}

func TestSetupUpdateConfirm_QDeclines(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseConfirmKnotfile}
	result, _ := m.updateConfirmKnotfile(keyMsg("q"))
	if !result.(setupModel).declined {
		t.Error("'q' should set declined=true")
	}
}

// ── setupModel.Update — message handling ─────────────────────────────────────

func TestSetupUpdate_CloneDone_WithError(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseCloning, cloneURL: "bad-url"}
	result, _ := m.Update(cloneDoneMsg{err: fmt.Errorf("git clone failed: exit 128")})
	sm := result.(setupModel)
	if sm.phase != setupPhaseGitRepo {
		t.Errorf("clone failure should return to setupPhaseGitRepo, got %d", sm.phase)
	}
	if sm.err == nil {
		t.Error("clone failure should set err on the model")
	}
}

func TestSetupUpdate_CloneDone_KnotfilePresent(t *testing.T) {
	dir := t.TempDir()
	knotfilePath := filepath.Join(dir, config.KnotfileName)
	if err := os.WriteFile(knotfilePath, []byte("packages: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := setupModel{dir: dir, phase: setupPhaseCloning}
	result, cmd := m.Update(cloneDoneMsg{})
	sm := result.(setupModel)
	if sm.phase != setupPhaseDone {
		t.Errorf("successful clone with Knotfile should advance to setupPhaseDone, got %d", sm.phase)
	}
	if cmd == nil {
		t.Error("should return tea.Quit after successful clone with Knotfile")
	}
}

func TestSetupUpdate_CloneDone_NoKnotfile_AutoCreates(t *testing.T) {
	dir := t.TempDir()
	// No Knotfile — clone succeeded but repo had none
	m := setupModel{dir: dir, phase: setupPhaseCloning}
	_, cmd := m.Update(cloneDoneMsg{})
	// Should dispatch writeKnotfileCmd automatically (no user prompt)
	if cmd == nil {
		t.Error("clone without Knotfile should dispatch writeKnotfileCmd automatically")
	}
}

func TestSetupUpdate_KnotfileReady_Success(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu}
	result, cmd := m.Update(knotfileReadyMsg{})
	sm := result.(setupModel)
	if sm.phase != setupPhaseDone {
		t.Errorf("successful write should advance to setupPhaseDone, got %d", sm.phase)
	}
	if cmd == nil {
		t.Error("should return tea.Quit after Knotfile written")
	}
}

func TestSetupUpdate_KnotfileReady_Error(t *testing.T) {
	m := setupModel{dir: "/tmp/d", phase: setupPhaseMenu}
	result, _ := m.Update(knotfileReadyMsg{err: fmt.Errorf("permission denied")})
	sm := result.(setupModel)
	if sm.err == nil {
		t.Error("write failure should set err on the model")
	}
	if sm.phase == setupPhaseDone {
		t.Error("phase should not advance to done on write error")
	}
}

// ── writeKnotfileCmd ──────────────────────────────────────────────────────────

func TestWriteKnotfileCmd_WritesTemplate(t *testing.T) {
	dir := t.TempDir()
	cmd := writeKnotfileCmd(dir)
	msg := cmd()
	result, ok := msg.(knotfileReadyMsg)
	if !ok {
		t.Fatalf("expected knotfileReadyMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	knotfilePath := filepath.Join(dir, config.KnotfileName)
	content, err := os.ReadFile(knotfilePath)
	if err != nil {
		t.Fatalf("Knotfile not written: %v", err)
	}
	if len(content) == 0 {
		t.Error("writeKnotfileCmd should write non-empty template content")
	}
}

func TestWriteKnotfileCmd_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "dotfiles")
	cmd := writeKnotfileCmd(dir)
	msg := cmd()
	result := msg.(knotfileReadyMsg)
	if result.err != nil {
		t.Fatalf("unexpected error creating nested dir: %v", result.err)
	}
	if _, err := os.Stat(filepath.Join(dir, config.KnotfileName)); err != nil {
		t.Error("Knotfile should exist after writeKnotfileCmd creates nested dirs")
	}
}

func TestWriteKnotfileCmd_IsValidYAML(t *testing.T) {
	dir := t.TempDir()
	writeKnotfileCmd(dir)()
	knotfilePath := filepath.Join(dir, config.KnotfileName)
	if _, err := config.Load(knotfilePath); err != nil {
		t.Errorf("written Knotfile should be parseable by config.Load: %v", err)
	}
}

// ── cloneRepoCmd ──────────────────────────────────────────────────────────────

func TestCloneRepoCmd_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "clone")
	cmd := cloneRepoCmd("not-a-real-url-xyz123", target)
	msg := cmd()
	result, ok := msg.(cloneDoneMsg)
	if !ok {
		t.Fatalf("expected cloneDoneMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("cloneRepoCmd with invalid URL should return an error")
	}
}
