package resolver

import (
	"path/filepath"
	"strings"

	"github.com/oxgrad/knot/internal/config"
)

// ExpandPath replaces a leading ~ with homeDir.
func ExpandPath(p, homeDir string) string {
	if p == "~" {
		return homeDir
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir, p[2:])
	}
	return p
}

// EvaluateCondition returns true if the package condition is satisfied for the given goos.
// A nil condition always passes.
func EvaluateCondition(cond *config.Condition, goos string) bool {
	if cond == nil {
		return true
	}
	if cond.OS != "" && cond.OS != goos {
		return false
	}
	return true
}

// ShouldIgnore returns true if the filename matches any of the given glob patterns.
// Patterns are matched against the base name of filename only — path separator
// components are stripped before matching, so patterns like "lua/*.lua" will
// never match regardless of the full path.
func ShouldIgnore(filename string, patterns []string) (bool, error) {
	base := filepath.Base(filename)
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, base)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
