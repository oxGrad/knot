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

func TestLoad_DefaultSource(t *testing.T) {
	dir := t.TempDir()
	yml := "packages:\n  nvim:\n    target: ~/.config/nvim\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := filepath.Join(dir, "nvim")
	if cfg.Packages["nvim"].Source != want {
		t.Errorf("default source = %q, want %q", cfg.Packages["nvim"].Source, want)
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

func TestLoad_Tags(t *testing.T) {
	dir := t.TempDir()
	yml := `packages:
  nvim:
    target: ~/.config/nvim
    tags: [work, linux]
  zsh:
    target: ~/
    tags: [home]
  secrets:
    target: ~/.ssh
`
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.Packages["nvim"].Tags; len(got) != 2 || got[0] != "work" || got[1] != "linux" {
		t.Errorf("nvim tags = %v, want [work linux]", got)
	}
	if got := cfg.Packages["secrets"].Tags; len(got) != 0 {
		t.Errorf("secrets tags = %v, want []", got)
	}
}

func TestPackagesByTag_Basic(t *testing.T) {
	cfg := &Config{
		Packages: map[string]Package{
			"nvim":    {Tags: []string{"work", "linux"}},
			"tmux":    {Tags: []string{"work"}},
			"zsh":     {Tags: []string{"home"}},
			"secrets": {},
		},
	}
	got := PackagesByTag(cfg)

	if pkgs := got["work"]; len(pkgs) != 2 || pkgs[0] != "nvim" || pkgs[1] != "tmux" {
		t.Errorf("work = %v, want [nvim tmux]", pkgs)
	}
	if pkgs := got["linux"]; len(pkgs) != 1 || pkgs[0] != "nvim" {
		t.Errorf("linux = %v, want [nvim]", pkgs)
	}
	if pkgs := got["home"]; len(pkgs) != 1 || pkgs[0] != "zsh" {
		t.Errorf("home = %v, want [zsh]", pkgs)
	}
	if _, ok := got["secrets"]; ok {
		t.Error("untagged package should not appear in PackagesByTag")
	}
}

func TestPackagesByTag_SortedPackages(t *testing.T) {
	cfg := &Config{
		Packages: map[string]Package{
			"zsh":  {Tags: []string{"home"}},
			"nvim": {Tags: []string{"home"}},
		},
	}
	got := PackagesByTag(cfg)
	if pkgs := got["home"]; len(pkgs) != 2 || pkgs[0] != "nvim" || pkgs[1] != "zsh" {
		t.Errorf("home = %v, want [nvim zsh] (sorted)", pkgs)
	}
}

func TestPackagesByTag_Empty(t *testing.T) {
	cfg := &Config{Packages: map[string]Package{}}
	if got := PackagesByTag(cfg); len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestLoadOne_ResolvesSourceRelativeToItself(t *testing.T) {
	dir := t.TempDir()
	yml := "packages:\n  nvim:\n    target: ~/.config/nvim\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadOne(path)
	if err != nil {
		t.Fatalf("loadOne() error: %v", err)
	}
	want := filepath.Join(dir, "nvim")
	if cfg.Packages["nvim"].Source != want {
		t.Errorf("source = %q, want %q", cfg.Packages["nvim"].Source, want)
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

func TestLoad_Include_Basic(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "work")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	rootKnot := "include:\n  - ./work\npackages:\n  zsh:\n    target: ~/\n"
	subKnot := "packages:\n  nvim:\n    target: ~/.config/nvim\n"

	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Knotfile"), []byte(subKnot), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(root, "Knotfile"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d: %v", len(cfg.Packages), cfg.Packages)
	}
	if _, ok := cfg.Packages["nvim"]; !ok {
		t.Error("expected nvim package from included file")
	}
	if _, ok := cfg.Packages["zsh"]; !ok {
		t.Error("expected zsh package from top-level")
	}
}

func TestLoad_Include_TopLevelWins(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "work")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	rootKnot := "include:\n  - ./work\npackages:\n  nvim:\n    target: ~/.config/nvim-top\n"
	subKnot := "packages:\n  nvim:\n    target: ~/.config/nvim-sub\n"

	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Knotfile"), []byte(subKnot), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(root, "Knotfile"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.Packages["nvim"].Target; got != "~/.config/nvim-top" {
		t.Errorf("nvim target = %q, want %q (top-level should win)", got, "~/.config/nvim-top")
	}
	if len(cfg.Packages) != 1 {
		t.Errorf("expected 1 package, got %d: %v", len(cfg.Packages), cfg.Packages)
	}
}

func TestLoad_Include_FirstWins(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	if err := os.MkdirAll(a, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0755); err != nil {
		t.Fatal(err)
	}

	rootKnot := "include:\n  - ./a\n  - ./b\n"
	aKnot := "packages:\n  nvim:\n    target: ~/.config/nvim-a\n"
	bKnot := "packages:\n  nvim:\n    target: ~/.config/nvim-b\n"

	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "Knotfile"), []byte(aKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "Knotfile"), []byte(bKnot), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(root, "Knotfile"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.Packages["nvim"].Target; got != "~/.config/nvim-a" {
		t.Errorf("nvim target = %q, want %q (first include should win)", got, "~/.config/nvim-a")
	}
	if len(cfg.Packages) != 1 {
		t.Errorf("expected 1 package, got %d: %v", len(cfg.Packages), cfg.Packages)
	}
}

func TestLoad_Include_NestedIgnored(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "work")
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	rootKnot := "include:\n  - ./work\npackages:\n  zsh:\n    target: ~/\n"
	subKnot := "include:\n  - ../nested\npackages:\n  nvim:\n    target: ~/.config/nvim\n"
	nestedKnot := "packages:\n  tmux:\n    target: ~/.tmux.conf\n"

	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Knotfile"), []byte(subKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "Knotfile"), []byte(nestedKnot), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(root, "Knotfile"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := cfg.Packages["tmux"]; ok {
		t.Error("tmux should not appear — nested includes must be ignored")
	}
	if len(cfg.Packages) != 2 {
		t.Errorf("expected 2 packages (zsh, nvim), got %d: %v", len(cfg.Packages), cfg.Packages)
	}
}

func TestLoad_Include_DirectoryMissing(t *testing.T) {
	root := t.TempDir()
	rootKnot := "include:\n  - ./nonexistent\n"
	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(root, "Knotfile"))
	if err == nil {
		t.Error("expected error when included directory has no Knotfile")
	}
}

func TestLoad_Include_NoKnotfile(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "empty")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	// directory exists but has no Knotfile inside it
	rootKnot := "include:\n  - ./empty\n"
	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(root, "Knotfile"))
	if err == nil {
		t.Error("expected error when included directory has no Knotfile")
	}
}

func TestLoad_Include_RelativePaths(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "work")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	rootKnot := "include:\n  - ./work\n"
	subKnot := "packages:\n  nvim:\n    target: ~/.config/nvim\n"

	if err := os.WriteFile(filepath.Join(root, "Knotfile"), []byte(rootKnot), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "Knotfile"), []byte(subKnot), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(root, "Knotfile"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := filepath.Join(sub, "nvim")
	if got := cfg.Packages["nvim"].Source; got != want {
		t.Errorf("nvim source = %q, want %q (should be relative to sub dir)", got, want)
	}
}

func TestLoadOne_Install_AutoResolveFillsManagerNames(t *testing.T) {
	dir := t.TempDir()
	yml := "packages:\n  nvim:\n    target: ~/.config/nvim\n    install:\n      bin: nvim\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadOne(path)
	if err != nil {
		t.Fatalf("loadOne() error: %v", err)
	}
	inst := cfg.Packages["nvim"].Install
	if inst == nil {
		t.Fatal("expected Install to be set")
	}
	if inst.Brew != "nvim" || inst.Apt != "nvim" || inst.Dnf != "nvim" {
		t.Errorf("expected brew/apt/dnf to default to %q, got brew=%q apt=%q dnf=%q", "nvim", inst.Brew, inst.Apt, inst.Dnf)
	}
}

func TestLoadOne_Install_AutoResolveFalseLeavesManagersEmpty(t *testing.T) {
	dir := t.TempDir()
	yml := "packages:\n  secrets:\n    target: ~/.ssh\n    install:\n      bin: age\n      script: https://example.com/install.sh\n      autoResolve: false\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadOne(path)
	if err != nil {
		t.Fatalf("loadOne() error: %v", err)
	}
	inst := cfg.Packages["secrets"].Install
	if inst == nil {
		t.Fatal("expected Install to be set")
	}
	if inst.Brew != "" || inst.Apt != "" || inst.Dnf != "" {
		t.Errorf("expected brew/apt/dnf to stay empty with autoResolve: false, got brew=%q apt=%q dnf=%q", inst.Brew, inst.Apt, inst.Dnf)
	}
}

func TestLoadOne_Install_AutoResolveDoesNotOverrideExplicit(t *testing.T) {
	dir := t.TempDir()
	yml := "packages:\n  neovim:\n    target: ~/.config/nvim\n    install:\n      brew: neovim-nightly\n"
	path := filepath.Join(dir, "Knotfile")
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadOne(path)
	if err != nil {
		t.Fatalf("loadOne() error: %v", err)
	}
	inst := cfg.Packages["neovim"].Install
	if inst.Brew != "neovim-nightly" {
		t.Errorf("explicit brew value overwritten: got %q, want %q", inst.Brew, "neovim-nightly")
	}
	if inst.Apt != "neovim" || inst.Dnf != "neovim" {
		t.Errorf("expected apt/dnf to still default to package name, got apt=%q dnf=%q", inst.Apt, inst.Dnf)
	}
}
