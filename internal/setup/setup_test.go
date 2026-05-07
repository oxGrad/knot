package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/oxgrad/knot/internal/config"
)

// testTemplate is a minimal valid Knotfile for tests.
var testTemplate = []byte("packages: {}\n")

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

func containsSubstr(s, sub string) bool {
	return strings.Contains(s, sub)
}

func newModel(dir string, mode Mode, ph phase) model {
	return model{dir: dir, mode: mode, phase: ph, knotfileTemplate: testTemplate}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func TestSetupModel_Init_ModeInit_ReturnsNil(t *testing.T) {
	m := newModel("/tmp/dotfiles", ModeInit, phaseMenu)
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil for ModeInit")
	}
}

func TestSetupModel_Init_ModeKnotfile_ReturnsCmd(t *testing.T) {
	m := newModel("/tmp/dotfiles", ModeKnotfile, phaseConfirmKnotfile)
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() should return non-nil cmd for ModeKnotfile")
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func TestSetupView_Menu(t *testing.T) {
	m := newModel("/home/user/.dotfiles", ModeInit, phaseMenu)
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
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 0, knotfileTemplate: testTemplate}
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
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 1, knotfileTemplate: testTemplate}
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
	m := model{dir: "/tmp/d", phase: phaseGitProvider, knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "GitHub") {
		t.Error("provider view should show GitHub option")
	}
	if !containsSubstr(out, "GitLab") {
		t.Error("provider view should show GitLab option")
	}
}

func TestSetupView_GitProtocol_GitHub(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProtocol, gitProvider: "github", knotfileTemplate: testTemplate}
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
	if !containsSubstr(strings.ToLower(out), "ssh") {
		t.Error("protocol view should mention SSH key requirement")
	}
}

func TestSetupView_GitProtocol_GitLab(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProtocol, gitProvider: "gitlab", knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "gitlab.com") {
		t.Error("protocol view should show gitlab.com when provider is gitlab")
	}
}

func TestSetupView_GitUsername(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "oxGrad", gitProvider: "github", knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "oxGrad") {
		t.Error("username view should show the current input buffer")
	}
	if !containsSubstr(out, "█") {
		t.Error("username view should show block cursor")
	}
}

func TestSetupView_GitRepo(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitRepo, inputBuf: "", knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "█") {
		t.Error("repo view should show block cursor")
	}
	if !containsSubstr(out, ".dotfiles") {
		t.Error("repo view should hint at the default repo name")
	}
}

func TestSetupView_GitRepo_ShowsError(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitRepo, err: fmt.Errorf("git clone failed: exit status 128"), knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "git clone failed") {
		t.Error("repo view should show error message")
	}
}

func TestSetupView_Cloning(t *testing.T) {
	m := model{
		dir:              "/home/user/.dotfiles",
		phase:            phaseCloning,
		cloneURL:         "https://github.com/user/dots",
		knotfileTemplate: testTemplate,
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
	m := model{dir: "/home/user/.dotfiles", phase: phaseConfirmKnotfile, knotfileTemplate: testTemplate}
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
	m := model{dir: "/tmp/d", phase: phaseConfirmKnotfile, err: fmt.Errorf("writing Knotfile: permission denied"), knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "writing Knotfile") {
		t.Error("confirm view should show error when present")
	}
}

func TestSetupView_Done(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseDone, knotfileTemplate: testTemplate}
	out := m.View()
	if !containsSubstr(out, "Setup complete") {
		t.Error("done view should say 'Setup complete'")
	}
}

func TestSetupView_AllPhases_NonEmpty(t *testing.T) {
	phases := []phase{
		phaseMenu,
		phaseGitProvider,
		phaseGitProtocol,
		phaseGitUsername,
		phaseGitRepo,
		phaseCloning,
		phaseConfirmKnotfile,
		phaseDone,
	}
	for _, ph := range phases {
		m := model{dir: "/tmp/d", phase: ph, knotfileTemplate: testTemplate}
		if out := m.View(); out == "" {
			t.Errorf("View() returned empty string for phase %d", ph)
		}
	}
}

// ── Update — WindowSizeMsg ────────────────────────────────────────────────────

func TestSetupUpdate_WindowSize(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, knotfileTemplate: testTemplate}
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	sm := result.(model)
	if sm.width != 120 {
		t.Errorf("expected width=120 after WindowSizeMsg, got %d", sm.width)
	}
}

// ── updateMenu ────────────────────────────────────────────────────────────────

func TestSetupUpdateMenu_NavigateDown(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 0, knotfileTemplate: testTemplate}
	result, _ := m.updateMenu(keyMsg("j"))
	if result.(model).cursor != 1 {
		t.Error("'j' should move cursor from 0 to 1")
	}
}

func TestSetupUpdateMenu_NavigateDownAtBottom(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 1, knotfileTemplate: testTemplate}
	result, _ := m.updateMenu(keyMsg("j"))
	if result.(model).cursor != 1 {
		t.Error("cursor should stay at 1 when already at bottom")
	}
}

func TestSetupUpdateMenu_NavigateUp(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 1, knotfileTemplate: testTemplate}
	result, _ := m.updateMenu(keyMsg("k"))
	if result.(model).cursor != 0 {
		t.Error("'k' should move cursor from 1 to 0")
	}
}

func TestSetupUpdateMenu_NavigateUpAtTop(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 0, knotfileTemplate: testTemplate}
	result, _ := m.updateMenu(keyMsg("k"))
	if result.(model).cursor != 0 {
		t.Error("cursor should stay at 0 when already at top")
	}
}

func TestSetupUpdateMenu_SelectInitNew(t *testing.T) {
	dir := t.TempDir()
	m := model{dir: dir, phase: phaseMenu, cursor: 0, knotfileTemplate: testTemplate}
	_, cmd := m.updateMenu(keyMsg("enter"))
	if cmd == nil {
		t.Error("selecting 'Initialize new' (cursor=0) should dispatch writeKnotfileCmd")
	}
}

func TestSetupUpdateMenu_SelectGitClone(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 1, knotfileTemplate: testTemplate}
	result, _ := m.updateMenu(keyMsg("enter"))
	sm := result.(model)
	if sm.phase != phaseGitProvider {
		t.Errorf("selecting option 1 should advance to phaseGitProvider, got phase %d", sm.phase)
	}
}

func TestSetupUpdateMenu_SpaceAlsoSelects(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, cursor: 1, knotfileTemplate: testTemplate}
	result, _ := m.updateMenu(keyMsg(" "))
	sm := result.(model)
	if sm.phase != phaseGitProvider {
		t.Error("space should also select the highlighted option")
	}
}

func TestSetupUpdateMenu_QuitKey(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, knotfileTemplate: testTemplate}
	_, cmd := m.updateMenu(keyMsg("q"))
	if cmd == nil {
		t.Error("'q' should return tea.Quit command")
	}
}

// ── updateGitProvider ─────────────────────────────────────────────────────────

func TestSetupUpdateGitProvider_SelectGitHub(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProvider, cursor: 0, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProvider(keyMsg("enter"))
	sm := result.(model)
	if sm.gitProvider != "github" {
		t.Errorf("cursor=0 should select github, got %q", sm.gitProvider)
	}
	if sm.phase != phaseGitProtocol {
		t.Errorf("should advance to phaseGitProtocol, got %d", sm.phase)
	}
}

func TestSetupUpdateGitProvider_SelectGitLab(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProvider, cursor: 1, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProvider(keyMsg("enter"))
	sm := result.(model)
	if sm.gitProvider != "gitlab" {
		t.Errorf("cursor=1 should select gitlab, got %q", sm.gitProvider)
	}
}

func TestSetupUpdateGitProvider_Navigate(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProvider, cursor: 0, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProvider(keyMsg("j"))
	if result.(model).cursor != 1 {
		t.Error("'j' should move cursor to 1")
	}
	result, _ = result.(model).updateGitProvider(keyMsg("k"))
	if result.(model).cursor != 0 {
		t.Error("'k' should move cursor back to 0")
	}
}

func TestSetupUpdateGitProvider_EscGoesBackToMenu(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProvider, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProvider(keyMsg("esc"))
	sm := result.(model)
	if sm.phase != phaseMenu {
		t.Errorf("esc should return to phaseMenu, got %d", sm.phase)
	}
	if sm.cursor != 1 {
		t.Error("esc should restore cursor to 1 (Clone option) in main menu")
	}
}

// ── updateGitProtocol ─────────────────────────────────────────────────────────

func TestSetupUpdateGitProtocol_SelectHTTPS(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProtocol, cursor: 0, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProtocol(keyMsg("enter"))
	sm := result.(model)
	if sm.gitProtocol != "https" {
		t.Errorf("cursor=0 should select https, got %q", sm.gitProtocol)
	}
	if sm.phase != phaseGitUsername {
		t.Errorf("should advance to phaseGitUsername, got %d", sm.phase)
	}
}

func TestSetupUpdateGitProtocol_SelectSSH(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProtocol, cursor: 1, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProtocol(keyMsg("enter"))
	sm := result.(model)
	if sm.gitProtocol != "ssh" {
		t.Errorf("cursor=1 should select ssh, got %q", sm.gitProtocol)
	}
}

func TestSetupUpdateGitProtocol_EscGoesBackToProvider(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitProtocol, knotfileTemplate: testTemplate}
	result, _ := m.updateGitProtocol(keyMsg("esc"))
	if result.(model).phase != phaseGitProvider {
		t.Error("esc should return to phaseGitProvider")
	}
}

// ── updateGitInput ────────────────────────────────────────────────────────────

func TestSetupUpdateGitInput_TypesCharacters(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "", knotfileTemplate: testTemplate}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, phaseGitProtocol, phaseGitRepo)
	result, _ = result.(model).updateGitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}, phaseGitProtocol, phaseGitRepo)
	if result.(model).inputBuf != "ab" {
		t.Errorf("expected inputBuf='ab', got %q", result.(model).inputBuf)
	}
}

func TestSetupUpdateGitInput_Backspace(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "abc", knotfileTemplate: testTemplate}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyBackspace}, phaseGitProtocol, phaseGitRepo)
	if result.(model).inputBuf != "ab" {
		t.Errorf("backspace should remove last char, got %q", result.(model).inputBuf)
	}
}

func TestSetupUpdateGitInput_BackspaceOnEmpty(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "", knotfileTemplate: testTemplate}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyBackspace}, phaseGitProtocol, phaseGitRepo)
	if result.(model).inputBuf != "" {
		t.Error("backspace on empty should be a no-op")
	}
}

func TestSetupUpdateGitInput_EnterUsername_AdvancesToRepo(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "oxGrad", knotfileTemplate: testTemplate}
	result, cmd := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, phaseGitProtocol, phaseGitRepo)
	sm := result.(model)
	if sm.gitUsername != "oxGrad" {
		t.Errorf("gitUsername should be set, got %q", sm.gitUsername)
	}
	if sm.phase != phaseGitRepo {
		t.Errorf("should advance to phaseGitRepo, got %d", sm.phase)
	}
	if cmd != nil {
		t.Error("advancing to repo phase should not dispatch any command")
	}
}

func TestSetupUpdateGitInput_EnterUsernameEmpty_IsNoop(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "   ", knotfileTemplate: testTemplate}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, phaseGitProtocol, phaseGitRepo)
	if result.(model).phase != phaseGitUsername {
		t.Error("blank username should not advance phase")
	}
}

func TestSetupUpdateGitInput_EnterRepo_StartsClone(t *testing.T) {
	m := model{
		dir: "/tmp/d", phase: phaseGitRepo,
		gitProvider: "github", gitProtocol: "https", gitUsername: "user",
		inputBuf:         "dotfiles",
		knotfileTemplate: testTemplate,
	}
	result, cmd := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, phaseGitUsername, phaseGitRepo)
	sm := result.(model)
	if sm.phase != phaseCloning {
		t.Errorf("should advance to phaseCloning, got %d", sm.phase)
	}
	if sm.cloneURL != "https://github.com/user/dotfiles" {
		t.Errorf("unexpected cloneURL: %q", sm.cloneURL)
	}
	if cmd == nil {
		t.Error("repo enter should dispatch cloneRepoCmd")
	}
}

func TestSetupUpdateGitInput_EnterRepoEmpty_UsesDefault(t *testing.T) {
	m := model{
		dir: "/tmp/d", phase: phaseGitRepo,
		gitProvider: "github", gitProtocol: "https", gitUsername: "user",
		inputBuf:         "",
		knotfileTemplate: testTemplate,
	}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEnter}, phaseGitUsername, phaseGitRepo)
	sm := result.(model)
	if sm.gitRepo != ".dotfiles" {
		t.Errorf("empty repo input should default to '.dotfiles', got %q", sm.gitRepo)
	}
	if sm.cloneURL != "https://github.com/user/.dotfiles" {
		t.Errorf("unexpected cloneURL: %q", sm.cloneURL)
	}
}

func TestSetupUpdateGitInput_EscGoesBack(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, inputBuf: "some text", err: fmt.Errorf("prev"), knotfileTemplate: testTemplate}
	result, _ := m.updateGitInput(tea.KeyMsg{Type: tea.KeyEsc}, phaseGitProtocol, phaseGitRepo)
	sm := result.(model)
	if sm.phase != phaseGitProtocol {
		t.Errorf("esc should go to backPhase (phaseGitProtocol), got %d", sm.phase)
	}
	if sm.inputBuf != "" {
		t.Error("esc should clear inputBuf")
	}
	if sm.err != nil {
		t.Error("esc should clear error")
	}
}

func TestSetupUpdateGitInput_CtrlCQuits(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseGitUsername, knotfileTemplate: testTemplate}
	_, cmd := m.updateGitInput(tea.KeyMsg{Type: tea.KeyCtrlC}, phaseGitProtocol, phaseGitRepo)
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

// ── updateConfirmKnotfile ─────────────────────────────────────────────────────

func TestSetupUpdateConfirm_YCreatesKnotfile(t *testing.T) {
	dir := t.TempDir()
	m := model{dir: dir, phase: phaseConfirmKnotfile, knotfileTemplate: testTemplate}
	_, cmd := m.updateConfirmKnotfile(keyMsg("y"))
	if cmd == nil {
		t.Error("'y' should dispatch writeKnotfileCmd")
	}
}

func TestSetupUpdateConfirm_EnterCreatesKnotfile(t *testing.T) {
	dir := t.TempDir()
	m := model{dir: dir, phase: phaseConfirmKnotfile, knotfileTemplate: testTemplate}
	_, cmd := m.updateConfirmKnotfile(keyMsg("enter"))
	if cmd == nil {
		t.Error("enter should dispatch writeKnotfileCmd")
	}
}

func TestSetupUpdateConfirm_NDeclines(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseConfirmKnotfile, knotfileTemplate: testTemplate}
	result, cmd := m.updateConfirmKnotfile(keyMsg("n"))
	sm := result.(model)
	if !sm.declined {
		t.Error("'n' should set declined=true")
	}
	if cmd == nil {
		t.Error("'n' should return tea.Quit")
	}
}

func TestSetupUpdateConfirm_EscDeclines(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseConfirmKnotfile, knotfileTemplate: testTemplate}
	result, cmd := m.updateConfirmKnotfile(keyMsg("esc"))
	sm := result.(model)
	if !sm.declined {
		t.Error("esc should set declined=true")
	}
	if cmd == nil {
		t.Error("esc should return tea.Quit")
	}
}

func TestSetupUpdateConfirm_QDeclines(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseConfirmKnotfile, knotfileTemplate: testTemplate}
	result, _ := m.updateConfirmKnotfile(keyMsg("q"))
	if !result.(model).declined {
		t.Error("'q' should set declined=true")
	}
}

// ── Update — message handling ─────────────────────────────────────────────────

func TestSetupUpdate_CloneDone_WithError(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseCloning, cloneURL: "bad-url", knotfileTemplate: testTemplate}
	result, _ := m.Update(cloneDoneMsg{err: fmt.Errorf("git clone failed: exit 128")})
	sm := result.(model)
	if sm.phase != phaseGitRepo {
		t.Errorf("clone failure should return to phaseGitRepo, got %d", sm.phase)
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
	m := model{dir: dir, phase: phaseCloning, knotfileTemplate: testTemplate}
	result, cmd := m.Update(cloneDoneMsg{})
	sm := result.(model)
	if sm.phase != phaseDone {
		t.Errorf("successful clone with Knotfile should advance to phaseDone, got %d", sm.phase)
	}
	if cmd == nil {
		t.Error("should return tea.Quit after successful clone with Knotfile")
	}
}

func TestSetupUpdate_CloneDone_NoKnotfile_AutoCreates(t *testing.T) {
	dir := t.TempDir()
	m := model{dir: dir, phase: phaseCloning, knotfileTemplate: testTemplate}
	_, cmd := m.Update(cloneDoneMsg{})
	if cmd == nil {
		t.Error("clone without Knotfile should dispatch writeKnotfileCmd automatically")
	}
}

func TestSetupUpdate_KnotfileReady_Success(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, knotfileTemplate: testTemplate}
	result, cmd := m.Update(knotfileReadyMsg{})
	sm := result.(model)
	if sm.phase != phaseDone {
		t.Errorf("successful write should advance to phaseDone, got %d", sm.phase)
	}
	if cmd == nil {
		t.Error("should return tea.Quit after Knotfile written")
	}
}

func TestSetupUpdate_KnotfileReady_Error(t *testing.T) {
	m := model{dir: "/tmp/d", phase: phaseMenu, knotfileTemplate: testTemplate}
	result, _ := m.Update(knotfileReadyMsg{err: fmt.Errorf("permission denied")})
	sm := result.(model)
	if sm.err == nil {
		t.Error("write failure should set err on the model")
	}
	if sm.phase == phaseDone {
		t.Error("phase should not advance to done on write error")
	}
}

// ── writeKnotfileCmd ──────────────────────────────────────────────────────────

func TestWriteKnotfileCmd_WritesTemplate(t *testing.T) {
	dir := t.TempDir()
	m := model{knotfileTemplate: testTemplate}
	cmd := m.writeKnotfileCmd(dir)
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
	m := model{knotfileTemplate: testTemplate}
	cmd := m.writeKnotfileCmd(dir)
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
	m := model{knotfileTemplate: testTemplate}
	m.writeKnotfileCmd(dir)()
	knotfilePath := filepath.Join(dir, config.KnotfileName)
	if _, err := config.Load(knotfilePath); err != nil {
		t.Errorf("written Knotfile should be parseable by config.Load: %v", err)
	}
}

// ── cloneRepoCmd ──────────────────────────────────────────────────────────────

func TestCloneRepoCmd_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "clone")
	m := model{knotfileTemplate: testTemplate}
	cmd := m.cloneRepoCmd("not-a-real-url-xyz123", target)
	msg := cmd()
	result, ok := msg.(cloneDoneMsg)
	if !ok {
		t.Fatalf("expected cloneDoneMsg, got %T", msg)
	}
	if result.err == nil {
		t.Error("cloneRepoCmd with invalid URL should return an error")
	}
}
