package linker

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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
		Writer:  &bytes.Buffer{},
	}
}

func TestPlan_CreateActions(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua":      "-- neovim config",
		"lua/plugin.lua": "-- plugin",
	})
	target := t.TempDir()

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
	if len(creates) != 2 {
		t.Errorf("expected 2 OpCreate actions, got %d", len(creates))
	}
}

func TestPlan_IgnorePatterns(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua":  "-- neovim config",
		"README.md": "# readme",
		".DS_Store": "junk",
	})
	target := t.TempDir()

	l := newTestLinker(false)
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source: source,
				Target: target,
				Ignore: []string{"README.md", ".DS_Store"},
			},
		},
	}

	actions, err := l.Plan(cfg, []string{"nvim"})
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	creates := filterOp(actions, OpCreate)
	if len(creates) != 1 {
		t.Errorf("expected 1 OpCreate (only init.lua), got %d", len(creates))
	}
	skips := filterOp(actions, OpSkip)
	if len(skips) != 2 {
		t.Errorf("expected 2 OpSkip, got %d", len(skips))
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
				Source: source,
				Target: target,
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
	target := t.TempDir()

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

	linkPath := filepath.Join(target, "init.lua")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}

	dest, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(source, "init.lua")
	if dest != expected {
		t.Errorf("symlink points to %q, want %q", dest, expected)
	}
}

func TestApply_Idempotent(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := t.TempDir()

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
	target := t.TempDir()

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
	linkPath := filepath.Join(target, "init.lua")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
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
	target := t.TempDir()

	// Pre-create a real file at the target location.
	linkPath := filepath.Join(target, "init.lua")
	if err := os.WriteFile(linkPath, []byte("existing file"), 0644); err != nil {
		t.Fatal(err)
	}

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

	// Original file should be untouched.
	content, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "existing file" {
		t.Error("conflict target should not be overwritten")
	}
}

func TestPlanUntie(t *testing.T) {
	source := makePackageTree(t, map[string]string{
		"init.lua": "-- config",
	})
	target := t.TempDir()

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
	target := t.TempDir()

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

	linkPath := filepath.Join(target, "init.lua")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("symlink should have been removed")
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

	_, err := l.Plan(cfg, []string{"nvim"})
	if err == nil {
		t.Error("expected error when source directory does not exist")
	}
}

func TestPlan_EmptySourceDir(t *testing.T) {
	source := t.TempDir() // empty directory
	target := t.TempDir()

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
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty source dir, got %d", len(actions))
	}
}

func TestPlan_BrokenSymlinkAtTarget(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	target := t.TempDir()

	// Create a broken symlink at the target location.
	linkPath := filepath.Join(target, "init.lua")
	if err := os.Symlink("/nonexistent/destination", linkPath); err != nil {
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
	target := t.TempDir()

	// Create a symlink pointing to a different real file.
	otherFile := filepath.Join(t.TempDir(), "other.lua")
	if err := os.WriteFile(otherFile, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(target, "init.lua")
	if err := os.Symlink(otherFile, linkPath); err != nil {
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
		"init.lua":          "-- top level",
		"lua/colors.lua":    "-- colors",
		"lua/lsp/init.lua":  "-- lsp",
	})
	target := t.TempDir()

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

	creates := filterOp(actions, OpCreate)
	if len(creates) != 3 {
		t.Errorf("expected 3 OpCreate for nested files, got %d", len(creates))
	}

	// Apply and verify intermediate directories are created.
	if err := l.Apply(actions); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	for _, rel := range []string{"init.lua", "lua/colors.lua", "lua/lsp/init.lua"} {
		path := filepath.Join(target, rel)
		info, err := os.Lstat(path)
		if err != nil {
			t.Errorf("expected symlink at %s: %v", path, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected symlink at %s, got regular file", path)
		}
	}
}

func TestPlanUntie_NoLinks(t *testing.T) {
	source := makePackageTree(t, map[string]string{"init.lua": "-- config"})
	target := t.TempDir()

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
	target := t.TempDir()

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
	linkPath := filepath.Join(target, "init.lua")
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
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

	// Before tying: both files should show as MISSING.
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
	target := t.TempDir()

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
