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

	_, err := l.Plan(cfg, []string{"nvim"})
	if err == nil {
		t.Error("expected error when source directory does not exist")
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
