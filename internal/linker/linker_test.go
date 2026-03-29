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

func filterOp(actions []LinkAction, op OpType) []LinkAction {
	var result []LinkAction
	for _, a := range actions {
		if a.Op == op {
			result = append(result, a)
		}
	}
	return result
}
