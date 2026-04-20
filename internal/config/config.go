package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	KnotfileName = "Knotfile"
	EnvKnotDir   = "KNOT_DIR"
)

// KnownOS is the set of accepted condition.os values.
var KnownOS = map[string]bool{
	"darwin": true, "linux": true, "windows": true, "freebsd": true,
}

// Config is the top-level structure parsed from Knotfile.
type Config struct {
	Packages map[string]Package `yaml:"packages"`
}

// Package describes one managed dotfile bundle.
type Package struct {
	Source    string     `yaml:"source"`
	Target    string     `yaml:"target"`
	Ignore    []string   `yaml:"ignore,omitempty"`
	Condition *Condition `yaml:"condition,omitempty"`
}

// Condition gates a package on runtime attributes.
type Condition struct {
	OS string `yaml:"os"`
}

// DefaultDir returns the dotfiles directory: $KNOT_DIR if set, else ~/. dotfiles.
func DefaultDir(homeDir string) string {
	if d := os.Getenv(EnvKnotDir); d != "" {
		return d
	}
	return filepath.Join(homeDir, ".dotfiles")
}

// DefaultKnotfilePath returns the default Knotfile path for the given home directory.
func DefaultKnotfilePath(homeDir string) string {
	return filepath.Join(DefaultDir(homeDir), KnotfileName)
}

// Load reads and parses a Knotfile at the given path.
//
// Path resolution in Load:
//   - Relative source paths (e.g. "./nvim") are resolved to absolute paths
//     relative to the Knotfile's directory so the linker always receives
//     absolute source paths.
//   - Paths beginning with "~/" (in both source and target) are intentionally
//     left as-is and expanded later by resolver.ExpandPath, which has access
//     to the runtime home directory.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	if cfg.Packages == nil {
		cfg.Packages = make(map[string]Package)
	}

	// Resolve relative source paths to the Knotfile's directory.
	dir := filepath.Dir(path)
	for name, pkg := range cfg.Packages {
		if pkg.Source != "" && !filepath.IsAbs(pkg.Source) && pkg.Source[0] != '~' {
			pkg.Source = filepath.Join(dir, pkg.Source)
		}
		if pkg.Target != "" && !filepath.IsAbs(pkg.Target) && pkg.Target[0] != '~' {
			pkg.Target = filepath.Join(dir, pkg.Target)
		}
		cfg.Packages[name] = pkg
	}

	return &cfg, nil
}

// FindConfigFile walks upward from startDir looking for a Knotfile.
// Returns the absolute path to the first Knotfile found, or an error if none exists.
func FindConfigFile(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving start directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, KnotfileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("knotfile not found (searched from %q upward)", startDir)
		}
		dir = parent
	}
}
