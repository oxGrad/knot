package linker

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/oxgrad/knot/internal/config"
)

// makePackageTree creates a temporary directory with the given files and returns its path.
func makePackageTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newTestLinker(dryRun bool) *Linker {
	return &Linker{
		DryRun:  dryRun,
		HomeDir: "/home/testuser",
		GOOS:    runtime.GOOS,
		GoArch:  runtime.GOARCH,
		Writer:  &bytes.Buffer{},
	}
}

func newTemplateLinker(dryRun bool) *Linker {
	return &Linker{
		DryRun:  dryRun,
		HomeDir: "/home/testuser",
		GOOS:    "linux",
		GoArch:  "amd64",
		Writer:  &bytes.Buffer{},
	}
}

func TestPlan_CreateActions(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua":       "-- neovim config",
		"lua/plugin.lua": "-- plugin",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source: source,
				Target: target,
			},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Errorf("expected 1 OpCreate action (directory symlink), got %d", len(creates))
	}
}

func TestPlan_IgnorePatterns(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua":  "-- neovim config",
		"README.md": "# readme",
		".DS_Store": "junk",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source: source,
				Target: target,
				Ignore: []string{"README.md", ".DS_Store"}, // kept for config compatibility
			},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	// With directory-level symlinking the whole source dir is linked as one — 1 create, 0 skips.
	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Errorf("expected 1 OpCreate (source directory), got %d", len(creates))
	}
	skips := filterOp(actions, OpSkip)
	if len(skips) != 0 {
		t.Errorf("expected 0 OpSkip, got %d", len(skips))
	}
}

func TestPlan_ConditionNotMet(t *testing.T) {
	source := makePackageTree(t, map[string]string{"file.conf": "data"})
	target := t.TempDir()

	l := &Linker{
		DryRun:  false,
		HomeDir: "/home/testuser",
		GOOS:    "linux", // not darwin
		Writer:  &bytes.Buffer{},
	}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"yabai": {
				Source:    source,
				Target:    target,
				Condition: &config.Condition{OS: "darwin"},
			},
		},
	}

	actions, err := l.Plan(cfg, []string{"yabai"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if len(actions) != 1 || actions[0].Op != OpSkip {
		t.Errorf("expected 1 OpSkip for unmet condition, got %+v", actions)
	}
}

func TestApply_CreatesSymlinks(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// target itself should now be a symlink pointing to source.
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file or directory")
	}

	dest, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if dest != source {
		t.Errorf("symlink points to %q, want %q", dest, source)
	}
}

func TestApply_Idempotent(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Apply twice — should succeed both times.
	for i := 0; i < 2; i++ {
		actions, err := l.Plan(cfg, []string{"nvim"})
		if err != nil {
			t.Fatalf("Plan() round %d error: %v", i+1, err)
		}
		if err := l.Apply(actions); err != nil {
			t.Fatalf("Apply() round %d error: %v", i+1, err)
		}
	}

	// Second plan should yield OpExists.
	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if a.Op != OpExists {
			t.Errorf("after second apply expected OpExists, got %s for %s", a.Op, a.Target)
		}
	}
}

func TestApply_DryRun(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	var buf bytes.Buffer
	l := &Linker{
		DryRun:  true,
		HomeDir: "/home/testuser",
		GOOS:    runtime.GOOS,
		Writer:  &buf,
	}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Apply(actions); err != nil {
		t.Fatal(err)
	}

	// No symlink should have been created.
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Error("dry-run should not create symlink")
	}

	// But output should mention it.
	if buf.Len() == 0 {
		t.Error("dry-run should produce output")
	}
}

func TestApply_Conflict(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	// target is a real existing directory — conflict with creating a directory symlink there.
	target := t.TempDir()

	var buf bytes.Buffer
	l := &Linker{
		DryRun:  false,
		HomeDir: "/home/testuser",
		GOOS:    runtime.GOOS,
		Writer:  &buf,
	}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatal(err)
	}

	conflicts := filterOp(actions, OpConflict)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	// Apply should not error, just warn.
	if err := l.Apply(actions); err != nil {
		t.Errorf("Apply() with conflict should not return error, got: %v", err)
	}

	// Target (real directory) should still exist and not be a symlink.
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("conflict target should not be replaced with a symlink")
	}
}

func TestPlanUntie(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// First tie.
	actions, _ := l.Plan(cfg, []string{"nvim"})
	_ = l.Apply(actions)

	// Now plan untie.
	untieActions, err := l.PlanUntie(cfg, []string{"nvim"})
	if err != nil {
		t.Fatal(err)
	}

	removes := filterOp(untieActions, OpRemove)
	if len(removes) != 1 {
		t.Errorf("expected 1 OpRemove, got %d", len(removes))
	}
}

func TestApply_Remove(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Tie first.
	actions, _ := l.Plan(cfg, []string{"nvim"})
	_ = l.Apply(actions)

	// Untie.
	untieActions, _ := l.PlanUntie(cfg, []string{"nvim"})
	if err := l.Apply(untieActions); err != nil {
		t.Fatalf("Apply() untie error: %v", err)
	}

	// The directory symlink should have been removed.
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Error("directory symlink should have been removed")
	}
}

func TestPlan_UnknownPackage(t *testing.T) {
	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: t.TempDir(), Target: t.TempDir()},
		},
	}

	_, err := l.Plan(cfg, []string{"does-not-exist"})
	if err == nil {
		t.Error("expected error for unknown package name")
	}
}

func TestPlan_SourceNotExist(t *testing.T) {
	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source: "/nonexistent/path/that/does/not/exist",
				Target: t.TempDir(),
			},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 || actions[0].Op != OpSourceNotFound {
		t.Errorf("expected single OpSourceNotFound action, got %v", actions)
	}
}

func TestPlan_EmptySourceDir(t *testing.T) {
	source := t.TempDir() // empty directory
	target := filepath.Join(t.TempDir(), "empty-target")

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"empty": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"empty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	// Even an empty source dir produces 1 action: symlink the directory itself.
	if len(actions) != 1 {
		t.Errorf("expected 1 action for empty source dir, got %d", len(actions))
	}
	if actions[0].Op != OpCreate {
		t.Errorf("expected OpCreate, got %s", actions[0].Op)
	}
}

func TestPlan_BrokenSymlinkAtTarget(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	// Put the broken symlink at the target path itself.
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "nvim-config")
	if err := os.Symlink("/nonexistent/destination", target); err != nil {
		t.Fatal(err)
	}

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	brokens := filterOp(actions, OpBroken)
	if len(brokens) != 1 {
		t.Errorf("expected 1 OpBroken, got %d (actions: %+v)", len(brokens), actions)
	}
}

func TestPlan_WrongSymlinkAtTarget(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	// Create a symlink at the target path pointing to a different real directory.
	otherDir := t.TempDir()
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "nvim-config")
	if err := os.Symlink(otherDir, target); err != nil {
		t.Fatal(err)
	}

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	conflicts := filterOp(actions, OpConflict)
	if len(conflicts) != 1 {
		t.Errorf("expected 1 OpConflict for wrong symlink, got %d (actions: %+v)", len(conflicts), actions)
	}
}

func TestPlan_NestedFiles(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua":         "-- top level",
		"lua/colors.lua":   "-- colors",
		"lua/lsp/init.lua": "-- lsp",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	// With directory-level symlinking, nested files produce one action for the root dir.
	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Errorf("expected 1 OpCreate (source directory symlink), got %d", len(creates))
	}

	// Apply and verify the directory symlink, plus that files are reachable through it.
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("expected symlink at %s: %v", target, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink at %s, got regular file or directory", target)
	}

	// Files should be reachable through the symlink.
	for _, rel := range []string{"init.lua", "lua/colors.lua", "lua/lsp/init.lua"} {
		path := filepath.Join(target, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file reachable at %s: %v", path, err)
		}
	}
}

func TestPlanUntie_NoLinks(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Nothing is tied yet — PlanUntie should produce zero OpRemove actions.
	actions, err := l.PlanUntie(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("PlanUntie() error: %v", err)
	}
	removes := filterOp(actions, OpRemove)
	if len(removes) != 0 {
		t.Errorf("expected 0 OpRemove when nothing is tied, got %d", len(removes))
	}
}

// TestPlanUntie_NotLinked_SkipsNotCreates verifies that PlanUntie never produces
// OpCreate actions — the root cause of the toggle bug where untie would link
// files that weren't yet linked.
func TestPlanUntie_NotLinked_SkipsNotCreates(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
		"lazy.lua": "-- lazy",
	})
	target := t.TempDir()

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Nothing tied — PlanUntie must not produce any OpCreate.
	actions, err := l.PlanUntie(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("PlanUntie() error: %v", err)
	}

	if creates := filterOp(actions, OpCreate); len(creates) != 0 {
		t.Errorf("PlanUntie produced %d OpCreate actions, want 0 (toggle bug)", len(creates))
	}
}

// TestApply_UntieDoesNotToggle is the end-to-end regression test for the toggle
// bug: tie → untie → untie again must not re-create the symlink.
func TestApply_UntieDoesNotToggle(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	target := t.TempDir()
	linkPath := filepath.Join(target, "init.lua")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Tie.
	actions, _ := l.Plan(cfg, []string{"nvim"})
	_ = l.Apply(actions)

	// Untie.
	untieActions, _ := l.PlanUntie(cfg, []string{"nvim"})
	_ = l.Apply(untieActions)

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatal("symlink should not exist after first untie")
	}

	// Untie again — must not re-create the symlink.
	untieActions2, err := l.PlanUntie(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("second PlanUntie() error: %v", err)
	}
	if err := l.Apply(untieActions2); err != nil {
		t.Fatalf("second Apply() error: %v", err)
	}

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("untie toggled the symlink back — bug is present")
	}
}

func TestApply_DryRunRemove(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	target := filepath.Join(t.TempDir(), "nvim-config")

	l := newTestLinker(false)
	l.Writer = &bytes.Buffer{}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Tie first.
	actions, _ := l.Plan(cfg, []string{"nvim"})
	_ = l.Apply(actions)

	// Switch to dry-run mode and untie.
	var buf bytes.Buffer
	l.DryRun = true
	l.Writer = &buf

	untieActions, _ := l.PlanUntie(cfg, []string{"nvim"})
	if err := l.Apply(untieActions); err != nil {
		t.Fatalf("Apply() dry-run remove error: %v", err)
	}

	// Symlink should still exist.
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		t.Error("dry-run remove should not delete the symlink")
	}
	// But output should mention it.
	if buf.Len() == 0 {
		t.Error("dry-run remove should produce output")
	}
}

func TestStatus_Output(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
		"lazy.lua": "-- lazy",
	})
	target := filepath.Join(t.TempDir(), "nvim-config")

	var buf bytes.Buffer
	l := &Linker{
		DryRun:  false,
		HomeDir: "/home/testuser",
		GOOS:    runtime.GOOS,
		Writer:  &buf,
	}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	// Before tying: directory symlink target is missing.
	if err := l.Status(cfg); err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	out := buf.String()
	if !containsString(out, "[MISSING]") {
		t.Errorf("status output before tie should contain [MISSING], got:\n%s", out)
	}

	// Tie, then check again.
	buf.Reset()
	actions, _ := l.Plan(cfg, []string{"nvim"})
	_ = l.Apply(actions)

	if err := l.Status(cfg); err != nil {
		t.Fatalf("Status() after tie error: %v", err)
	}
	out = buf.String()
	if !containsString(out, "[OK]") {
		t.Errorf("status output after tie should contain [OK], got:\n%s", out)
	}
}

func TestPrintPlan_Output(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	target := filepath.Join(t.TempDir(), "nvim-config")

	var buf bytes.Buffer
	l := &Linker{
		DryRun:  true,
		HomeDir: "/home/testuser",
		GOOS:    runtime.GOOS,
		Writer:  &buf,
	}

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatal(err)
	}
	l.PrintPlan(actions)

	out := buf.String()
	if !containsString(out, "+") {
		t.Errorf("PrintPlan output should contain '+' for new symlinks, got:\n%s", out)
	}
	if !containsString(out, "Plan:") {
		t.Errorf("PrintPlan output should contain summary line starting with 'Plan:', got:\n%s", out)
	}
}

// TestOpType_String verifies all OpType string representations.
func TestOpType_String(t *testing.T) {
	cases := []struct {
		op   OpType
		want string
	}{
		{OpCreate, "create"},
		{OpExists, "exists"},
		{OpConflict, "conflict"},
		{OpBroken, "broken"},
		{OpRemove, "remove"},
		{OpSkip, "skip"},
		{OpType(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.op.String(); got != c.want {
			t.Errorf("OpType(%d).String() = %q, want %q", c.op, got, c.want)
		}
	}
}

// TestNew verifies the New constructor sets sensible defaults.
func TestNew(t *testing.T) {
	l := New(true)
	if l == nil {
		t.Fatal("New() returned nil")
	}
	if !l.DryRun {
		t.Error("expected DryRun=true")
	}
	if l.HomeDir == "" {
		t.Error("expected HomeDir to be non-empty")
	}
	if l.GOOS == "" {
		t.Error("expected GOOS to be non-empty")
	}
	if l.Writer == nil {
		t.Error("expected Writer to be non-nil")
	}
}

// TestPrintPlan_AllOps covers all OpType branches in PrintPlan.
func TestPrintPlan_AllOps(t *testing.T) {
	source := makePackageTree(t, map[string]string{"f.lua": "-- x"})
	target := t.TempDir()

	var buf bytes.Buffer
	l := &Linker{Writer: &buf}

	actions := []LinkAction{
		{Op: OpCreate, Source: source, Target: filepath.Join(target, "create.lua")},
		{Op: OpRemove, Target: filepath.Join(target, "remove.lua")},
		{Op: OpExists, Target: filepath.Join(target, "exists.lua")},
		{Op: OpConflict, Target: filepath.Join(target, "conflict.lua"), Reason: "some reason"},
		{Op: OpBroken, Target: filepath.Join(target, "broken.lua")},
		{Op: OpSkip, Target: filepath.Join(target, "skip.lua")},
	}
	l.PrintPlan(actions)

	out := buf.String()
	checks := []string{"+", "-", "=", "!", "~", "Plan:"}
	for _, want := range checks {
		if !containsString(out, want) {
			t.Errorf("PrintPlan output missing %q:\n%s", want, out)
		}
	}
}

// TestPrintPlan_Summary verifies counts in the summary line.
func TestPrintPlan_Summary(t *testing.T) {
	var buf bytes.Buffer
	l := &Linker{Writer: &buf}
	l.PrintPlan([]LinkAction{
		{Op: OpCreate},
		{Op: OpCreate},
		{Op: OpRemove},
		{Op: OpExists},
		{Op: OpConflict},
	})
	out := buf.String()
	if !containsString(out, "2 to create") {
		t.Errorf("expected '2 to create' in output:\n%s", out)
	}
	if !containsString(out, "1 to remove") {
		t.Errorf("expected '1 to remove' in output:\n%s", out)
	}
}

// TestStatus_ConflictAndBroken verifies Status reports conflict and broken lines.
// With directory-level symlinking, CONFLICT means the target path already exists
// as a non-symlink, and BROKEN means it is a symlink pointing nowhere.
// Two packages are needed to produce both outcomes simultaneously.
func TestStatus_ConflictAndBroken(t *testing.T) {
	source1 := makePackageTree(t, map[string]string{"conf.lua": "-- config"})
	source2 := makePackageTree(t, map[string]string{"other.lua": "-- other"})

	// Conflict: target path is an existing directory (not a symlink).
	conflictTarget := t.TempDir()

	// Broken: target path is a symlink pointing nowhere.
	brokenTarget := filepath.Join(t.TempDir(), "broken")
	if err := os.Symlink("/nonexistent/path", brokenTarget); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	l := &Linker{
		DryRun:  false,
		HomeDir: "/home/testuser",
		GOOS:    "linux",
		Writer:  &buf,
	}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: conflictTarget},
			"vim":  {Source: source2, Target: brokenTarget},
		},
	}

	if err := l.Status(cfg); err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	out := buf.String()
	if !containsString(out, "[CONFLICT]") {
		t.Errorf("expected [CONFLICT] in status output:\n%s", out)
	}
	if !containsString(out, "[BROKEN]") {
		t.Errorf("expected [BROKEN] in status output:\n%s", out)
	}
}

// TestApply_BrokenAndConflict verifies Apply outputs [BROKEN] and [CONFLICT] lines.
// Two packages are used: one whose target is a real directory (CONFLICT) and one
// whose target is a broken symlink (BROKEN).
func TestApply_BrokenAndConflict(t *testing.T) {
	source1 := makePackageTree(t, map[string]string{"conf.lua": "-- config"})
	source2 := makePackageTree(t, map[string]string{"other.lua": "-- other"})

	// Conflict: target path is an existing directory.
	conflictTarget := t.TempDir()

	// Broken: target path is a symlink pointing nowhere.
	brokenTarget := filepath.Join(t.TempDir(), "broken")
	if err := os.Symlink("/nonexistent", brokenTarget); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	l := &Linker{
		DryRun:  false,
		HomeDir: "/home/testuser",
		GOOS:    "linux",
		Writer:  &buf,
	}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: source1, Target: conflictTarget},
			"vim":  {Source: source2, Target: brokenTarget},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim", "vim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	out := buf.String()
	if !containsString(out, "[CONFLICT]") {
		t.Errorf("expected [CONFLICT] in apply output:\n%s", out)
	}
	if !containsString(out, "[BROKEN]") {
		t.Errorf("expected [BROKEN] in apply output:\n%s", out)
	}
}

// TestPlanPackage_SourceIsFile verifies an error when source is a file, not a dir.
func TestPlanPackage_SourceIsFile(t *testing.T) {
	// Write a regular file and use it as source.
	f, err := os.CreateTemp(t.TempDir(), "source-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"bad": {Source: f.Name(), Target: t.TempDir()},
		},
	}
	_, err = l.Plan(cfg, []string{"bad"})
	if err == nil {
		t.Error("expected error when source is a file, not a directory")
	}
}

// TestPlan_PerFileMode verifies that a target ending with "/" triggers per-file
// linking: each source entry gets its own OpCreate action inside the target dir.
func TestPlan_PerFileMode(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		".zshrc":  "# zsh",
		".zshenv": "# env",
	})
	target := t.TempDir() // pre-existing directory, like ~/

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			// trailing slash signals per-file mode
			"zsh": {Source: source, Target: target + "/"},
		},
	}

	actions, err := l.Plan(cfg, []string{"zsh"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	creates := filterOp(actions, OpCreate)
	if len(creates) != 2 {
		t.Errorf("expected 2 OpCreate (one per file), got %d: %+v", len(creates), actions)
	}
	for _, a := range creates {
		if a.Source == source || a.Target == target {
			t.Errorf("per-file action points at whole directory: %+v", a)
		}
	}
}

// TestPlan_PerFileMode_Ignore verifies that ignore patterns are applied in per-file mode.
func TestPlan_PerFileMode_Ignore(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		".zshrc":    "# zsh",
		"README.md": "# docs",
	})
	target := t.TempDir()

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"zsh": {
				Source: source,
				Target: target + "/",
				Ignore: []string{"README.md"},
			},
		},
	}

	actions, err := l.Plan(cfg, []string{"zsh"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Errorf("expected 1 OpCreate (.zshrc only), got %d: %+v", len(creates), actions)
	}
	skips := filterOp(actions, OpSkip)
	if len(skips) != 1 {
		t.Errorf("expected 1 OpSkip (README.md), got %d: %+v", len(skips), actions)
	}
}

// TestPlan_PerFileMode_Conflict verifies that a pre-existing file at the target
// location is reported as OpConflict in per-file mode.
func TestPlan_PerFileMode_Conflict(t *testing.T) {
	source := makePackageTree(t, map[string]string{".zshrc": "# zsh"})
	target := t.TempDir()

	// pre-existing regular file at the would-be symlink location
	if err := os.WriteFile(filepath.Join(target, ".zshrc"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"zsh": {Source: source, Target: target + "/"},
		},
	}

	actions, err := l.Plan(cfg, []string{"zsh"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	conflicts := filterOp(actions, OpConflict)
	if len(conflicts) != 1 {
		t.Errorf("expected 1 OpConflict for pre-existing file, got %d: %+v", len(conflicts), actions)
	}
}

// containsString is a helper to avoid importing strings in test assertions.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func filterOp(actions []LinkAction, op OpType) []LinkAction {
	var result []LinkAction
	for _, a := range actions {
		if a.Op == op {
			result = append(result, a)
		}
	}
	return result
}

// --- Template tests ---

func TestPlan_PerFileMode_TemplateFile(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `font-size = {{ if eq .os "linux" }}9{{ else }}12{{ end }}`,
		"other.conf":  "plain file",
	})
	targetDir := t.TempDir() + "/"

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir},
		},
	}

	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	renders := filterOp(actions, OpRender)
	if len(renders) != 1 {
		t.Fatalf("expected 1 OpRender, got %d", len(renders))
	}
	if !strings.HasSuffix(renders[0].Target, "/config") {
		t.Errorf("render target = %q, want suffix /config (no .tmpl)", renders[0].Target)
	}
	if strings.HasSuffix(renders[0].Target, ".tmpl") {
		t.Errorf("render target must not have .tmpl suffix, got %q", renders[0].Target)
	}

	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Fatalf("expected 1 OpCreate for plain file, got %d", len(creates))
	}
	if !strings.HasSuffix(creates[0].Target, "/other.conf") {
		t.Errorf("create target = %q, want suffix /other.conf", creates[0].Target)
	}
}

func TestPlan_PerFileMode_TemplateAlreadyRendered(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `font-size = 9`,
	})
	targetDir := t.TempDir()

	// Pre-write the exact rendered content.
	targetFile := filepath.Join(targetDir, "config")
	if err := os.WriteFile(targetFile, []byte("font-size = 9"), 0644); err != nil {
		t.Fatal(err)
	}

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	renderExists := filterOp(actions, OpRenderExists)
	if len(renderExists) != 1 {
		t.Errorf("expected 1 OpRenderExists, got %d (actions: %+v)", len(renderExists), actions)
	}
}

func TestPlan_PerFileMode_TemplateContentChanged(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `font-size = 9`,
	})
	targetDir := t.TempDir()

	// Pre-write stale content.
	targetFile := filepath.Join(targetDir, "config")
	if err := os.WriteFile(targetFile, []byte("font-size = 12"), 0644); err != nil {
		t.Fatal(err)
	}

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	renders := filterOp(actions, OpRender)
	if len(renders) != 1 {
		t.Errorf("expected 1 OpRender for changed content, got %d", len(renders))
	}
}

func TestApply_RendersTemplate(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `font-size = {{ if eq .os "linux" }}9{{ else }}12{{ end }}`,
	})
	targetDir := t.TempDir()

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	targetFile := filepath.Join(targetDir, "config")
	info, err := os.Lstat(targetFile)
	if err != nil {
		t.Fatalf("target file not found: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("target must be a regular file, not a symlink")
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(content) != "font-size = 9" {
		t.Errorf("rendered content = %q, want %q", string(content), "font-size = 9")
	}

	// Permissions should be 0644.
	if info.Mode().Perm() != 0o644 {
		t.Errorf("file permissions = %o, want 0644", info.Mode().Perm())
	}
}

func TestApply_RenderDryRun(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `value = {{ .os }}`,
	})
	targetDir := t.TempDir()

	buf := &bytes.Buffer{}
	l := newTemplateLinker(true)
	l.Writer = buf

	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// Target file must not be created in dry-run.
	targetFile := filepath.Join(targetDir, "config")
	if _, err := os.Stat(targetFile); err == nil {
		t.Error("target file must not exist after dry-run apply")
	}
	if !strings.Contains(buf.String(), "[dry-run] render") {
		t.Errorf("expected dry-run output, got %q", buf.String())
	}
}

func TestPlanUntie_RenderedFile(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `value = {{ .os }}`,
	})
	targetDir := t.TempDir()

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	// First: tie (render the template).
	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// Now plan the untie.
	untieActions, err := l.PlanUntie(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("PlanUntie() error: %v", err)
	}

	removes := filterOp(untieActions, OpRemoveRendered)
	if len(removes) != 1 {
		t.Errorf("expected 1 OpRemoveRendered, got %d (actions: %+v)", len(removes), untieActions)
	}
}

func TestApply_RemoveRendered(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `value = {{ .os }}`,
	})
	targetDir := t.TempDir()

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	// Tie.
	actions, _ := l.Plan(cfg, []string{"ghostty"})
	_ = l.Apply(actions)

	targetFile := filepath.Join(targetDir, "config")
	if _, err := os.Stat(targetFile); err != nil {
		t.Fatalf("rendered file should exist after tie: %v", err)
	}

	// Untie.
	untieActions, err := l.PlanUntie(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("PlanUntie() error: %v", err)
	}
	if err := l.Apply(untieActions); err != nil {
		t.Fatalf("Apply(untie) error: %v", err)
	}

	if _, err := os.Stat(targetFile); err == nil {
		t.Error("rendered file should be removed after untie")
	}
}

func TestApply_RenderIdempotent(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `value = {{ .os }}`,
	})
	targetDir := t.TempDir()

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: targetDir + "/"},
		},
	}

	// First apply.
	actions, _ := l.Plan(cfg, []string{"ghostty"})
	_ = l.Apply(actions)

	// Second plan: should show OpRenderExists.
	actions2, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("second Plan() error: %v", err)
	}
	renderExists := filterOp(actions2, OpRenderExists)
	if len(renderExists) != 1 {
		t.Errorf("expected 1 OpRenderExists on second plan, got %d (actions: %+v)", len(renderExists), actions2)
	}
}

func TestPlan_DirectoryMode_IgnoresTemplates(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"config.tmpl": `value = {{ .os }}`,
		"other.conf":  "plain",
	})
	// No trailing slash → directory mode.
	target := filepath.Join(t.TempDir(), "ghostty-dir")

	l := newTemplateLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"ghostty": {Source: source, Target: target},
		},
	}

	actions, err := l.Plan(cfg, []string{"ghostty"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	// Directory mode: single OpCreate for the whole directory, no OpRender.
	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Errorf("expected 1 OpCreate (directory symlink), got %d", len(creates))
	}
	renders := filterOp(actions, OpRender)
	if len(renders) != 0 {
		t.Errorf("directory mode must not produce OpRender, got %d", len(renders))
	}
}

func TestOpType_String_NewValues(t *testing.T) {
	cases := []struct {
		op   OpType
		want string
	}{
		{OpRender, "render"},
		{OpRenderExists, "render-exists"},
		{OpRemoveRendered, "remove-rendered"},
	}
	for _, tc := range cases {
		if got := tc.op.String(); got != tc.want {
			t.Errorf("OpType(%d).String() = %q, want %q", tc.op, got, tc.want)
		}
	}
}
