package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure parsed from Knotfile.
type Config struct {
	Packages map[string]Package `yaml:"packages"`
	Include  []string           `yaml:"include,omitempty"`
}

// Package describes one managed dotfile bundle.
type Package struct {
	Source    string     `yaml:"source"`
	Target    string     `yaml:"target"`
	Ignore    []string   `yaml:"ignore,omitempty"`
	Condition *Condition `yaml:"condition,omitempty"`
	Tags      []string   `yaml:"tags,omitempty"`
	Install   *Install   `yaml:"install,omitempty"`
}

// Condition gates a package on runtime attributes.
type Condition struct {
	OS string `yaml:"os"`
}

// Install holds optional application installation metadata for a package.
type Install struct {
	Bin    string   `yaml:"bin,omitempty"`
	Brew   string   `yaml:"brew,omitempty"`
	Apt    string   `yaml:"apt,omitempty"`
	Dnf    string   `yaml:"dnf,omitempty"`
	Script string   `yaml:"script,omitempty"`
	Deps   []string `yaml:"deps,omitempty"`

	// AutoResolve fills any of Brew/Apt/Dnf left empty with the Knotfile
	// package name. Defaults to true; set to false to require each
	// manager field to be listed explicitly.
	AutoResolve *bool `yaml:"autoResolve,omitempty"`
}

// autoResolveEnabled reports whether empty manager fields should default to
// the package name. Unset (nil) defaults to true.
func (i *Install) autoResolveEnabled() bool {
	return i.AutoResolve == nil || *i.AutoResolve
}

// loadOne reads and parses a single Knotfile at path, resolving source paths
// relative to the file's directory. It does not process Include entries.
func loadOne(path string) (*Config, error) {
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

	dir := filepath.Dir(path)
	for name, pkg := range cfg.Packages {
		if pkg.Source == "" {
			pkg.Source = "./" + name
		}
		if !filepath.IsAbs(pkg.Source) {
			pkg.Source = filepath.Join(dir, pkg.Source)
		}
		if pkg.Install != nil && pkg.Install.autoResolveEnabled() {
			if pkg.Install.Brew == "" {
				pkg.Install.Brew = name
			}
			if pkg.Install.Apt == "" {
				pkg.Install.Apt = name
			}
			if pkg.Install.Dnf == "" {
				pkg.Install.Dnf = name
			}
		}
		cfg.Packages[name] = pkg
	}

	return &cfg, nil
}

// Load reads and parses a Knotfile at the given path, merging packages from
// any directories listed under include:. Top-level packages take priority over
// included ones; among included files the first listed wins.
func Load(path string) (*Config, error) {
	top, err := loadOne(path)
	if err != nil {
		return nil, err
	}

	topDir := filepath.Dir(path)
	for _, incDir := range top.Include {
		if !filepath.IsAbs(incDir) {
			incDir = filepath.Join(topDir, incDir)
		}
		incPath := filepath.Join(incDir, KnotfileName)
		inc, err := loadOne(incPath)
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", incDir, err)
		}
		for name, pkg := range inc.Packages {
			if _, exists := top.Packages[name]; !exists {
				top.Packages[name] = pkg
			}
		}
	}

	top.Include = nil
	return top, nil
}

// KnotfileName is the canonical name of the configuration file.
const KnotfileName = "Knotfile"

// EnvKnotDir is the environment variable that overrides the default dotfiles directory.
const EnvKnotDir = "KNOT_DIR"

// DefaultDir returns the dotfiles directory: $KNOT_DIR if set, otherwise homeDir/.dotfiles.
func DefaultDir(homeDir string) string {
	if d := os.Getenv(EnvKnotDir); d != "" {
		return d
	}
	return filepath.Join(homeDir, ".dotfiles")
}

// DefaultKnotfilePath returns the default path to the Knotfile for the given home directory.
func DefaultKnotfilePath(homeDir string) string {
	return filepath.Join(DefaultDir(homeDir), KnotfileName)
}

// PackagesByTag returns a map of tag name → sorted slice of package names.
// Only packages that declare at least one tag are included.
func PackagesByTag(cfg *Config) map[string][]string {
	result := make(map[string][]string)
	for name, pkg := range cfg.Packages {
		for _, tag := range pkg.Tags {
			result[tag] = append(result[tag], name)
		}
	}
	for tag := range result {
		sort.Strings(result[tag])
	}
	return result
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
