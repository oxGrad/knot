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
	path := filepath.Join(dir, "knot.yml")
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
	path := filepath.Join(dir, "knot.yml")
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
	path := filepath.Join(dir, "knot.yml")
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

	// Place knot.yml in root
	knotPath := filepath.Join(root, "knot.yml")
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
		t.Error("expected error when knot.yml not found")
	}
}
