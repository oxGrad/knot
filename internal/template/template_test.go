package template

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTmplFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.tmpl")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func baseData() TemplateData {
	return TemplateData{
		"os":       "linux",
		"arch":     "amd64",
		"hostname": "testhost",
		"username": "testuser",
		"home":     "/home/testuser",
		"env":      map[string]string{},
	}
}

func TestRenderFile_BasicSubstitution(t *testing.T) {
	src := writeTmplFile(t, "os={{ .os }} arch={{ .arch }}")
	got, err := RenderFile(src, baseData())
	if err != nil {
		t.Fatalf("RenderFile() error: %v", err)
	}
	if string(got) != "os=linux arch=amd64" {
		t.Errorf("got %q, want %q", string(got), "os=linux arch=amd64")
	}
}

func TestRenderFile_ConditionalBlock_Darwin(t *testing.T) {
	src := writeTmplFile(t, `{{ if eq .os "darwin" }}font-size = 12{{ else }}font-size = 9{{ end }}`)
	data := baseData()
	data["os"] = "darwin"

	got, err := RenderFile(src, data)
	if err != nil {
		t.Fatalf("RenderFile() error: %v", err)
	}
	if string(got) != "font-size = 12" {
		t.Errorf("darwin: got %q, want %q", string(got), "font-size = 12")
	}
}

func TestRenderFile_ConditionalBlock_Linux(t *testing.T) {
	src := writeTmplFile(t, `{{ if eq .os "darwin" }}font-size = 12{{ else }}font-size = 9{{ end }}`)

	got, err := RenderFile(src, baseData())
	if err != nil {
		t.Fatalf("RenderFile() error: %v", err)
	}
	if string(got) != "font-size = 9" {
		t.Errorf("linux: got %q, want %q", string(got), "font-size = 9")
	}
}

func TestRenderFile_EnvVariable(t *testing.T) {
	src := writeTmplFile(t, `val={{ index .env "MY_VAR" }}`)
	data := baseData()
	data["env"] = map[string]string{"MY_VAR": "hello"}

	got, err := RenderFile(src, data)
	if err != nil {
		t.Fatalf("RenderFile() error: %v", err)
	}
	if string(got) != "val=hello" {
		t.Errorf("got %q, want %q", string(got), "val=hello")
	}
}

func TestRenderFile_ParseError(t *testing.T) {
	src := writeTmplFile(t, `{{ .os `)
	_, err := RenderFile(src, baseData())
	if err == nil {
		t.Error("expected parse error, got nil")
	}
}

func TestRenderFile_FileNotFound(t *testing.T) {
	_, err := RenderFile("/nonexistent/path/test.tmpl", baseData())
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestBuildTemplateData_Fields(t *testing.T) {
	data, err := BuildTemplateData("linux", "amd64", "/home/testuser")
	if err != nil {
		t.Fatalf("BuildTemplateData() error: %v", err)
	}
	if data["os"] != "linux" {
		t.Errorf("os = %q, want %q", data["os"], "linux")
	}
	if data["arch"] != "amd64" {
		t.Errorf("arch = %q, want %q", data["arch"], "amd64")
	}
	if data["home"] != "/home/testuser" {
		t.Errorf("home = %q, want %q", data["home"], "/home/testuser")
	}
	if data["hostname"] == "" {
		t.Error("hostname should not be empty")
	}
	if data["username"] == "" {
		t.Error("username should not be empty")
	}
	if _, ok := data["env"].(map[string]string); !ok {
		t.Errorf("env should be map[string]string, got %T", data["env"])
	}
}

func TestBuildTemplateData_EnvMap(t *testing.T) {
	t.Setenv("KNOT_TEST_VAR", "testvalue")

	data, err := BuildTemplateData("linux", "amd64", "/home/testuser")
	if err != nil {
		t.Fatalf("BuildTemplateData() error: %v", err)
	}

	env, ok := data["env"].(map[string]string)
	if !ok {
		t.Fatalf("env is not map[string]string")
	}
	if env["KNOT_TEST_VAR"] != "testvalue" {
		t.Errorf("env[KNOT_TEST_VAR] = %q, want %q", env["KNOT_TEST_VAR"], "testvalue")
	}
}
