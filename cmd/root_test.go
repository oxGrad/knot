package cmd

import (
	"testing"

	"github.com/oxgrad/knot/internal/config"
)

func TestResolvePackageArgs_All(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {},
			"zsh":  {},
		},
	}
	names, err := resolvePackageArgs(nil, true, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 packages, got %d", len(names))
	}
}

func TestResolvePackageArgs_Specific(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {},
			"zsh":  {},
		},
	}
	names, err := resolvePackageArgs([]string{"nvim"}, false, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "nvim" {
		t.Errorf("expected [nvim], got %v", names)
	}
}

func TestResolvePackageArgs_UnknownPackage(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {},
		},
	}
	_, err := resolvePackageArgs([]string{"nonexistent"}, false, cfg)
	if err == nil {
		t.Error("expected error for unknown package name")
	}
}

func TestResolvePackageArgs_EmptyConfig_All(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{},
	}
	names, err := resolvePackageArgs(nil, true, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 packages, got %d", len(names))
	}
}

func TestResolvePackageArgs_EmptyArgs(t *testing.T) {
	cfg := &config.Config{
		Packages: map[string]config.Package{
			"nvim": {},
		},
	}
	names, err := resolvePackageArgs([]string{}, false, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty args returned unchanged, got %v", names)
	}
}
