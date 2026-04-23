package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oxgrad/knot/internal/config"
)

func TestRunValidation_EmptyPackages(t *testing.T) {
	cfg := &config.Config{Packages: map[string]config.Package{}}
	errs, warns := runValidation(cfg, t.TempDir())

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(warns) != 1 || warns[0] != "no packages defined" {
		t.Errorf("expected 'no packages defined' warning, got %v", warns)
	}
}

func TestRunValidation_ValidPackage(t *testing.T) {
	src := t.TempDir()
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: src, Target: "~/.config/nvim"},
		},
	}
	errs, warns := runValidation(cfg, t.TempDir())

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings, got %v", warns)
	}
}

func TestRunValidation_MissingSource(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: "", Target: "~/.config/nvim"},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0] != `[nvim]: "source" is required` {
		t.Errorf("unexpected error message: %s", errs[0])
	}
}

func TestRunValidation_SourceDoesNotExist(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: "/nonexistent/path/to/nvim", Target: "~/.config/nvim"},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "does not exist") {
		t.Errorf("expected 'does not exist' in error, got: %s", errs[0])
	}
}

func TestRunValidation_SourceIsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: f, Target: "~/.config/nvim"},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "not a directory") {
		t.Errorf("expected 'not a directory' in error, got: %s", errs[0])
	}
}

func TestRunValidation_MissingTarget(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: t.TempDir(), Target: ""},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0] != `[nvim]: "target" is required` {
		t.Errorf("unexpected error message: %s", errs[0])
	}
}

func TestRunValidation_BothFieldsMissing(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"broken": {Source: "", Target: ""},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 2 {
		t.Errorf("expected 2 errors (missing source + target), got %d: %v", len(errs), errs)
	}
}

func TestRunValidation_UnknownConditionOS(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source:    t.TempDir(),
				Target:    "~/.config/nvim",
				Condition: &config.Condition{OS: "haiku"},
			},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "haiku") {
		t.Errorf("expected OS name in error, got: %s", errs[0])
	}
}

func TestRunValidation_KnownConditionOS(t *testing.T) {
	src := t.TempDir()
	for _, goos := range []string{"darwin", "linux", "windows", "freebsd"} {
		cfg := &config.Config{
			Packages: map[string]config.Package{
				"pkg": {
					Source:    src,
					Target:    "~/target",
					Condition: &config.Condition{OS: goos},
				},
			},
		}
		errs, _ := runValidation(cfg, t.TempDir())
		if len(errs) != 0 {
			t.Errorf("os=%s: expected no errors, got %v", goos, errs)
		}
	}
}

func TestRunValidation_InvalidIgnorePattern(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source: t.TempDir(),
				Target: "~/.config/nvim",
				Ignore: []string{"[invalid-bracket"},
			},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "invalid ignore pattern") {
		t.Errorf("expected 'invalid ignore pattern' in error, got: %s", errs[0])
	}
}

func TestRunValidation_ValidIgnorePatterns(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {
				Source: t.TempDir(),
				Target: "~/.config/nvim",
				Ignore: []string{"*.md", ".DS_Store", "README*"},
			},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid glob patterns, got %v", errs)
	}
}

func TestRunValidation_HomeTildeExpansion(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(home, "dotfiles", "nvim")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {Source: "~/dotfiles/nvim", Target: "~/.config/nvim"},
		},
	}
	errs, _ := runValidation(cfg, home)
	if len(errs) != 0 {
		t.Errorf("expected ~/... to expand correctly, got errors: %v", errs)
	}
}

func TestRunValidation_SortedErrors(t *testing.T) {
	// Three packages with missing sources — errors must appear alphabetically.
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"zsh":  {Source: "/nope/z", Target: "~/zsh"},
			"nvim": {Source: "/nope/n", Target: "~/nvim"},
			"git":  {Source: "/nope/g", Target: "~/git"},
		},
	}
	errs, _ := runValidation(cfg, t.TempDir())

	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
	for i, pkg := range []string{"[git]", "[nvim]", "[zsh]"} {
		if !strings.Contains(errs[i], pkg) {
			t.Errorf("errs[%d] = %q, expected to contain %q", i, errs[i], pkg)
		}
	}
}

func TestRunValidation_NoConditionOS(t *testing.T) {
	// nil Condition and empty OS should both pass without error.
	src := t.TempDir()
	for _, cond := range []*config.Condition{nil, {OS: ""}} {
		cfg := &config.Config{
			Packages: map[string]config.Package{
				"pkg": {Source: src, Target: "~/target", Condition: cond},
			},
		}
		errs, _ := runValidation(cfg, t.TempDir())
		if len(errs) != 0 {
			t.Errorf("condition=%v: expected no errors, got %v", cond, errs)
		}
	}
}
