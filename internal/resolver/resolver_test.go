package resolver

import (
	"testing"

	"github.com/oxgrad/knot/internal/config"
)

func TestExpandPath(t *testing.T) {
	home := "/home/testuser"

	tests := []struct {
		input string
		want  string
	}{
		{"~", "/home/testuser"},
		{"~/", "/home/testuser"},
		{"~/.config/nvim", "/home/testuser/.config/nvim"},
		{"~/dotfiles", "/home/testuser/dotfiles"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ExpandPath(tt.input, home)
		if got != tt.want {
			t.Errorf("ExpandPath(%q, %q) = %q, want %q", tt.input, home, got, tt.want)
		}
	}
}

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		cond *config.Condition
		goos string
		want bool
	}{
		{nil, "linux", true},
		{nil, "darwin", true},
		{&config.Condition{OS: "darwin"}, "darwin", true},
		{&config.Condition{OS: "darwin"}, "linux", false},
		{&config.Condition{OS: "linux"}, "linux", true},
		{&config.Condition{OS: "linux"}, "darwin", false},
		{&config.Condition{OS: ""}, "linux", true},
		{&config.Condition{OS: ""}, "darwin", true},
	}

	for _, tt := range tests {
		got := EvaluateCondition(tt.cond, tt.goos)
		if got != tt.want {
			t.Errorf("EvaluateCondition(%+v, %q) = %v, want %v", tt.cond, tt.goos, got, tt.want)
		}
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		filename string
		patterns []string
		want     bool
		wantErr  bool
	}{
		{"README.md", []string{"README.md"}, true, false},
		{"README.md", []string{"*.md"}, true, false},
		{"init.lua", []string{"README.md", ".DS_Store"}, false, false},
		{"init.lua", []string{}, false, false},
		{".DS_Store", []string{".DS_Store"}, true, false},
		{"/some/path/README.md", []string{"README.md"}, true, false},
		{"file.txt", []string{"*.md", "*.DS_Store"}, false, false},
		{"test.md", []string{"*.md"}, true, false},
	}

	for _, tt := range tests {
		got, err := ShouldIgnore(tt.filename, tt.patterns)
		if (err != nil) != tt.wantErr {
			t.Errorf("ShouldIgnore(%q, %v) error = %v, wantErr %v", tt.filename, tt.patterns, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ShouldIgnore(%q, %v) = %v, want %v", tt.filename, tt.patterns, got, tt.want)
		}
	}
}
