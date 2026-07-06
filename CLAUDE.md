# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...
go build -o knot .

# Test
go test ./...
go test -v -coverprofile=coverage.out ./...   # with coverage (CI requires ≥60%)
go test -v ./internal/config                   # single package
go test -run TestLoad ./internal/config        # single test

# Lint
golangci-lint run

# Multi-distro testing via Docker
make test-ubuntu
make test-fedora
make test-all
```

## Architecture

Knot is a CLI + TUI dotfiles manager. The CLI commands are in `cmd/` using Cobra. The TUI uses the BubbleTea model/update/view pattern. Core symlink logic lives entirely in `internal/`.

### Package responsibilities

| Package | Role |
|---|---|
| `cmd/root.go` | Cobra setup, `loadConfig()`, package/tag arg resolution |
| `cmd/tie.go`, `untie.go`, `plan.go`, etc. | Individual command `RunE` handlers |
| `cmd/tui*.go` | BubbleTea TUI — model, update, views, async commands |
| `internal/config` | YAML parsing, `~` expansion, `KNOT_DIR` env var, default source paths |
| `internal/linker` | `Plan()`, `Apply()`, `Status()` — all symlink logic |
| `internal/resolver` | `ExpandPath()`, `EvaluateCondition()`, `ShouldIgnore()` |
| `internal/setup` | Interactive wizard used by `knot init` |

### Core data flow

`knot tie nvim` follows this path:

```
tieCmd.RunE
  → loadConfig()                     # parse Knotfile YAML, resolve paths
  → linker.Plan(cfg, names)          # compute []LinkAction without touching disk
      → check condition (OS match)
      → expand ~ in source/target
      → per package: classifyTarget() → OpCreate/OpExists/OpConflict/OpBroken
  → linker.Apply(actions)            # execute or --dry-run print
```

The `linker` package is the heart of knot. `Plan()` inspects disk state and returns a `[]LinkAction`. `Apply()` executes them. Both the CLI commands and the TUI's apply phase use this same pair.

### TUI architecture

The TUI (`cmd/tui_model.go`) is a BubbleTea state machine. The key enum is `tuiPhase`:

- `phaseList` — browsing packages/tags, `space` to toggle, `a` to apply
- `phaseConfirm` — shows plan preview, `y` to proceed
- `phaseApply` — executing async apply via `applyCmd()`
- `phaseResult` — shows outcome, any key returns to list
- `phaseBranch` / `phaseCheckout` — async git branch switching
- `phaseInstallSelect` — choose package manager for `install` block

Async operations (git info, version checks, apply) return typed message structs (`gitInfoMsg`, `versionCheckMsg`, `applyDoneMsg`, etc.) processed in `tui_update.go`'s `Update()`.

**Tab organization:** `tabPackages` is a flat list; `tabTags` is a collapsible nested tree built by `buildTagRows()` in `tui_rows.go`.

### Linking modes

- `target: ~/.config/nvim` → single directory symlink
- `target: ~/` (trailing `/`) → per-file mode: each file linked individually, `ignore` patterns applied, `.tmpl` files rendered via Go `text/template`

### Config path resolution

Relative source paths (e.g. `./nvim`) are resolved relative to the **Knotfile directory**, not cwd. `~` is expanded at runtime. The default source for a package named `foo` is `./foo`. The dotfiles root defaults to `$HOME/.dotfiles` and can be overridden with `KNOT_DIR`.

## Key files

- `cmd/tui_model.go` — model struct, all state enums, message types
- `cmd/tui_update.go` — `Update()` phase dispatch logic
- `internal/linker/linker.go` — `Plan()`, `Apply()`, `classifyTarget()`
- `internal/config/config.go` — `Config`/`Package`/`Install` structs, `Load()`
- `.github/workflows/ci.yml` — enforces 60% test coverage minimum