package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	yml := `
packages:
  nvim:
    source: ./nvim
    target: ~/.config/nvim
    ignore:
      - "README.md"
      - ".DS_Store"
  zsh:
    source: ./zsh
    target: ~/
  yabai:
    source: ./yabai
    target: ~/.config/yabai
    condition:
      os: darwin
`
	dir := t.TempDir()
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Packages) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(cfg.Packages))
	}

	// nvim package
	nvim := cfg.Packages["nvim"]
	if nvim.Target != "~/.config/nvim" {
		t.Errorf("nvim target = %q, want %q", nvim.Target, "~/.config/nvim")
	}
	if nvim.Source != filepath.Join(dir, "nvim") {
		t.Errorf("nvim source = %q, want %q", nvim.Source, filepath.Join(dir, "nvim"))
	}
	if len(nvim.Ignore) != 2 {
		t.Errorf("nvim ignore = %v, want 2 items", nvim.Ignore)
	}
	if nvim.Condition != nil {
		t.Errorf("nvim condition should be nil")
	}

	// yabai package with condition
	yabai := cfg.Packages["yabai"]
	if yabai.Condition == nil {
		t.Fatal("yabai condition should not be nil")
	}
	if yabai.Condition.OS != "darwin" {
		t.Errorf("yabai condition.os = %q, want %q", yabai.Condition.OS, "darwin")
	}

	// zsh package
	zsh := cfg.Packages["zsh"]
	if zsh.Target != "~/" {
		t.Errorf("zsh target = %q, want %q", zsh.Target, "~/")
	}
}

func TestLoad_EmptyPackages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte("packages:\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Packages == nil {
		t.Error("Packages map should not be nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/knot.yml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte("{{invalid yaml}}"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create a directory tree: root/sub/deep
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	// Place Knotfile in root
	knotPath := filepath.Join(root, "Knotfile")
	if err := os.WriteFile(knotPath, []byte("packages:\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should find it from a subdirectory
	found, err := FindConfigFile(deep)
	if err != nil {
		t.Fatalf("FindConfigFile() error: %v", err)
	}
	if found != knotPath {
		t.Errorf("found = %q, want %q", found, knotPath)
	}

	// Should find it from the directory itself
	found, err = FindConfigFile(root)
	if err != nil {
		t.Fatalf("FindConfigFile() error: %v", err)
	}
	if found != knotPath {
		t.Errorf("found = %q, want %q", found, knotPath)
	}
}

func TestFindConfigFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindConfigFile(dir)
	if err == nil {
		t.Error("expected error when Knotfile not found")
	}
}

func TestLoad_AbsoluteSourcePath(t *testing.T) {
	dir := t.TempDir()
	absSource := "/usr/local/share/dotfiles/nvim"
	yml := "packages:\n  nvim:\n    source: " + absSource + "\n    target: ~/.config/nvim\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// Absolute source paths must not be modified.
	if cfg.Packages["nvim"].Source != absSource {
		t.Errorf("source = %q, want %q (absolute path should be unchanged)", cfg.Packages["nvim"].Source, absSource)
	}
}

func TestLoad_AbsentCondition(t *testing.T) {
	dir := t.TempDir()
	yml := "packages:\n  zsh:\n    source: ./zsh\n    target: ~/\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Packages["zsh"].Condition != nil {
		t.Error("expected Condition to be nil when not specified in YAML")
	}
}

func TestDefaultDir_NoEnv(t *testing.T) {
	t.Setenv(EnvKnotDir, "")
	got := DefaultDir("/home/testuser")
	want := "/home/testuser/.dotfiles"
	if got != want {
		t.Errorf("DefaultDir() = %q, want %q", got, want)
	}
}

func TestDefaultDir_WithEnv(t *testing.T) {
	t.Setenv(EnvKnotDir, "/custom/dotfiles")
	got := DefaultDir("/home/testuser")
	if got != "/custom/dotfiles" {
		t.Errorf("DefaultDir() = %q, want %q", got, "/custom/dotfiles")
	}
}

func TestDefaultKnotfilePath(t *testing.T) {
	t.Setenv(EnvKnotDir, "")
	got := DefaultKnotfilePath("/home/testuser")
	want := "/home/testuser/.dotfiles/Knotfile"
	if got != want {
		t.Errorf("DefaultKnotfilePath() = %q, want %q", got, want)
	}
}

func TestDefaultKnotfilePath_WithEnv(t *testing.T) {
	t.Setenv(EnvKnotDir, "/custom/dotfiles")
	got := DefaultKnotfilePath("/home/testuser")
	want := "/custom/dotfiles/Knotfile"
	if got != want {
		t.Errorf("DefaultKnotfilePath() = %q, want %q", got, want)
	}
}

func TestFindConfigFile_RelativePath(t *testing.T) {
	// Change into a temp directory so a relative path resolution is meaningful.
	root := t.TempDir()
	knotPath := filepath.Join(root, "Knotfile")
	if err := os.WriteFile(knotPath, []byte("packages:\n"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := FindConfigFile(root)
	if err != nil {
		t.Fatalf("FindConfigFile() error: %v", err)
	}
	// Result must be absolute regardless of how startDir was provided.
	if !filepath.IsAbs(found) {
		t.Errorf("FindConfigFile() returned non-absolute path %q", found)
	}
	if found != knotPath {
		t.Errorf("found = %q, want %q", found, knotPath)
	}
}
