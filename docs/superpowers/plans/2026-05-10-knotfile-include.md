# Knotfile `include:` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `include:` top-level key to Knotfile that merges packages from referenced directory Knotfiles, with top-level and first-listed priority.

**Architecture:** Extract `loadOne()` from the current `Load()` to parse a single file without processing includes. `Load()` calls `loadOne()` for the top-level file, then iterates `include:` entries in order, merging non-conflicting packages into the result. All callers continue to receive a single flat `*Config` — no changes outside `internal/config/`.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, standard `os`/`filepath`.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/config/config.go` | Modify | Add `Include` field, extract `loadOne()`, update `Load()` |
| `internal/config/config_test.go` | Modify | Add 6 unit tests for include behaviour |
| `cmd/testdata/script/tie_include.txtar` | Create | End-to-end integration test |

---

### Task 1: Add `Include` field to `Config` and extract `loadOne()`

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Write a failing test for `loadOne` behaviour (source path resolution)**

Add to `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/config/... -run TestLoadOne_ResolvesSourceRelativeToItself -v
```

Expected: FAIL — `loadOne` undefined.

- [ ] **Step 3: Add `Include` field to `Config` and extract `loadOne()`**

Replace the current `Config` struct and `Load()` in `internal/config/config.go`:

```go
// Config is the top-level structure parsed from Knotfile.
type Config struct {
	Packages map[string]Package `yaml:"packages"`
	Include  []string           `yaml:"include,omitempty"`
}
```

Add `loadOne` just above `Load`:

```go
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
		cfg.Packages[name] = pkg
	}

	return &cfg, nil
}
```

Update `Load` to call `loadOne` and handle includes:

```go
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
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
go test ./internal/config/... -run TestLoadOne_ResolvesSourceRelativeToItself -v
```

Expected: PASS.

- [ ] **Step 5: Run full test suite to check no regressions**

```bash
go test ./internal/config/...
```

Expected: all existing tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add Include field and extract loadOne()"
```

---

### Task 2: Unit tests for include behaviour

**Files:**
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write all six failing tests**

Append to `internal/config/config_test.go`:

```go
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
	// sub includes nested — should be ignored
	subKnot := "include:\n  - ../nested\npackages:\n  nvim:\n    target: ~/.config/nvim\n"
	// nested has a package that must NOT appear (nested include ignored)
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

func TestLoad_Include_Missing(t *testing.T) {
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

func TestLoad_Include_RelativePaths(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "work")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	rootKnot := "include:\n  - ./work\n"
	// source omitted — should default to ./nvim relative to sub's dir
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
```

- [ ] **Step 2: Run tests to confirm they fail for the right reason**

```bash
go test ./internal/config/... -run "TestLoad_Include" -v
```

Expected: all FAIL — after Task 1, `Load()` is implemented, so these should actually PASS. If any fail, the implementation has a bug — fix before continuing.

- [ ] **Step 3: Run full config test suite**

```bash
go test ./internal/config/... -v
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test(config): add unit tests for include: behaviour"
```

---

### Task 3: Integration test

**Files:**
- Create: `cmd/testdata/script/tie_include.txtar`

- [ ] **Step 1: Write the integration test**

Create `cmd/testdata/script/tie_include.txtar`:

```
# knot tie merges packages from an included Knotfile directory.
env KNOT_DIR=$WORK/dotfiles

-- dotfiles/zsh/.zshrc --
# zsh config
-- dotfiles/work/nvim/init.lua --
-- lua config
-- dotfiles/work/Knotfile --
packages:
  nvim:
    source: ./nvim
    target: ~/nvim-config

-- dotfiles/Knotfile --
include:
  - ./work
packages:
  zsh:
    source: ./zsh
    target: ~/zsh-config

exec knot tie
stdout 'linked'

# Symlinks from top-level package must exist.
exists $HOME/zsh-config/.zshrc

# Symlinks from included package must exist.
exists $HOME/nvim-config/init.lua
```

- [ ] **Step 2: Run the integration test**

```bash
go test ./cmd/... -run TestScript/tie_include -v
```

Expected: PASS.

- [ ] **Step 3: Run full integration test suite**

```bash
go test ./cmd/...
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/testdata/script/tie_include.txtar
git commit -m "test(cmd): add integration test for Knotfile include:"
```

---

### Task 4: Final verification

- [ ] **Step 1: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 2: Build binary**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Manual smoke test**

```bash
mkdir -p /tmp/knot-test/dotfiles/work/nvim
echo "-- lua config" > /tmp/knot-test/dotfiles/work/nvim/init.lua
echo "packages:" > /tmp/knot-test/dotfiles/work/Knotfile
echo "  nvim:" >> /tmp/knot-test/dotfiles/work/Knotfile
echo "    source: ./nvim" >> /tmp/knot-test/dotfiles/work/Knotfile
echo "    target: ~/nvim-config-test" >> /tmp/knot-test/dotfiles/work/Knotfile

cat > /tmp/knot-test/dotfiles/Knotfile <<'EOF'
include:
  - ./work
packages:
  zsh:
    source: ./zsh
    target: ~/zsh-config-test
EOF

KNOT_DIR=/tmp/knot-test/dotfiles go run . plan
```

Expected: output shows both `zsh` and `nvim` packages.

- [ ] **Step 4: Cleanup smoke test artifacts**

```bash
rm -rf /tmp/knot-test ~/nvim-config-test ~/zsh-config-test
```
