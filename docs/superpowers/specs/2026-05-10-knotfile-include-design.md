# Knotfile `include:` Feature Design

**Date:** 2026-05-10

## Summary

Add `include:` as a top-level key in Knotfile. Each entry is a path to a directory containing a `Knotfile`. Packages from included files are merged into the top-level config, with the top-level taking priority on conflicts.

## Data Model

Add `Include []string` to `Config`:

```go
type Config struct {
    Packages map[string]Package `yaml:"packages"`
    Include  []string           `yaml:"include,omitempty"`
}
```

### Knotfile syntax

```yaml
include:
  - ./work
  - ./home

packages:
  zsh:
    target: ~/
```

Each `include:` entry is a directory path (relative or absolute). Relative paths are resolved against the including Knotfile's directory. `Knotfile` filename is appended automatically.

## Load Logic

Introduce `loadOne(path string) (*Config, error)` — parses a single Knotfile and resolves source paths relative to its own directory. Does not process `include:` entries.

`Load(path)` becomes:

```
Load(path):
  top = loadOne(path)
  merged = copy of top.Packages   // top-level wins all conflicts

  for each dir in top.Include (in order):
    incPath = filepath.Join(resolve(dir, top's dir), "Knotfile")
    inc = loadOne(incPath)         // inc.Include silently ignored
    for each pkg in inc.Packages:
      if pkg not in merged:
        merged[pkg] = inc.Packages[pkg]

  top.Include = nil                // strip from returned Config
  top.Packages = merged
  return top
```

### Priority rules

1. Top-level packages always win over any included package with the same name.
2. Among included files, first listed wins over later listed (stable, left-to-right).
3. Unique packages from included files are merged in.

### Error handling

- Included directory does not exist or contains no `Knotfile` → `Load()` returns an error (fail loud).
- Included Knotfile has invalid YAML → `Load()` returns an error.
- Nested `include:` in an included file → silently ignored (not an error).

### Source path resolution

Sources in an included Knotfile are resolved relative to that included file's directory, not the top-level Knotfile's directory.

## Changes Required

- `internal/config/config.go` — add `Include` field to `Config`, extract `loadOne()`, update `Load()`.
- `internal/config/config_test.go` — add unit tests (see Testing section).
- `cmd/testdata/script/tie_include.txtar` — integration test.

No changes required outside `internal/config/`.

## Testing

### Unit tests (`config_test.go`)

| Test | Verifies |
|------|----------|
| `TestLoad_Include_Basic` | Unique packages from included file appear in merged config |
| `TestLoad_Include_TopLevelWins` | Same package name in top-level and include: top-level preserved |
| `TestLoad_Include_FirstWins` | Same package name in two includes: first listed wins |
| `TestLoad_Include_NestedIgnored` | `include:` inside an included file is silently ignored |
| `TestLoad_Include_Missing` | Included dir has no Knotfile → error returned |
| `TestLoad_Include_RelativePaths` | Sources in included file resolved relative to included file's dir |

### Integration test (`tie_include.txtar`)

End-to-end `knot tie` with top-level Knotfile including a subdirectory Knotfile. Verifies symlinks from both top-level and included packages are created correctly.
