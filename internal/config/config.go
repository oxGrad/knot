package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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

// Load reads and parses a Knotfile at the given path.
// All relative paths inside the config are resolved relative to the config file's directory.
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

	// Resolve source paths relative to the config file's directory.
	dir := filepath.Dir(path)
	for name, pkg := range cfg.Packages {
		if pkg.Source != "" && !filepath.IsAbs(pkg.Source) {
			pkg.Source = filepath.Join(dir, pkg.Source)
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
		candidate := filepath.Join(dir, "Knotfile")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding the file.
			return "", fmt.Errorf("knotfile not found (searched from %q upward)", startDir)
		}
		dir = parent
	}
}
