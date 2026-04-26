package template

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateData holds variables available inside .tmpl files.
// Keys are lowercase to match the dot-access syntax users write (e.g. {{ .os }}).
type TemplateData map[string]any

// BuildTemplateData constructs template data from the current environment.
// goos and goarch are injected (rather than read from runtime) so callers can
// override them in tests without build tags.
func BuildTemplateData(goos, goarch, homeDir string) (TemplateData, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("getting hostname: %w", err)
	}

	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("getting current user: %w", err)
	}

	return TemplateData{
		"os":       goos,
		"arch":     goarch,
		"hostname": hostname,
		"username": u.Username,
		"home":     homeDir,
		"env":      buildEnvMap(),
	}, nil
}

// RenderFile reads srcPath as a Go text/template, executes it with data, and
// returns the rendered bytes.
func RenderFile(srcPath string, data TemplateData) ([]byte, error) {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("reading template %q: %w", srcPath, err)
	}

	tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", srcPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering template %q: %w", srcPath, err)
	}

	return buf.Bytes(), nil
}

func buildEnvMap() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, _ := strings.Cut(e, "=")
		env[k] = v
	}
	return env
}
