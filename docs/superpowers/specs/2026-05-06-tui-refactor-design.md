# TUI Refactor Design
_2026-05-06_

## Goal

Split `cmd/tui.go` (1957 lines) into focused files without changing any behaviour. Move the setup wizard to `internal/setup`. Add the brand header to the setup wizard view.

---

## File Map

### `cmd/` (package cmd)

| File | Responsibility | Est. lines |
|---|---|---|
| `tui.go` | `runTUI` entry point, `max`/`min` utils | ~60 |
| `tui_model.go` | Styles, all type/const defs, model struct, all `tea.Msg` types | ~130 |
| `tui_mascot.go` | `knotArt`, mascot frame data, `renderMascotLine` | ~115 |
| `tui_rows.go` | `pkgStatus`, `computeStatus`, `buildRows`, `buildTagRows`, `visibleTagItems`, toggle helpers | ~240 |
| `tui_cmds.go` | All `tea.Cmd` factories: `headerTickCmd`, `fetchGitInfoCmd`, `reloadCmd`, `applyCmd`, `editorCmd`, `fetchBranchesCmd`, `checkoutBranchCmd`, `gitPullCmd`, `dotfilesDir` | ~140 |
| `tui_update.go` | `Init`, `Update`, `updateList`, `updateConfirm`, `updateBranch`, `updateTags`, `buildConfirmLines` | ~290 |
| `tui_views.go` | `View`, `renderBrandHeader`, `viewList`, `viewTags`, `viewBranch`, `viewConfirm`, `viewResult`, layout helpers (`visibleHeight`, `adjustOffset`, etc.) | ~330 |

### `internal/setup/` (new package)

| File | Responsibility |
|---|---|
| `setup.go` | `Mode`, `Phase` types, `setupModel` struct, `Update`, `View`, `Init`, `Run` |

`Run(dir string, mode Mode) error` is the only exported symbol consumed by `cmd/tui.go`.

---

## Brand Header in Setup Wizard

`renderBrandHeader` stays in `package cmd` (`tui_views.go`). The setup wizard is launched from `runSetupWizard` inside `package cmd`, so the wrapper function passes a pre-rendered header string into the `setupModel` at construction time, or the `setupModel.View()` calls a header-render func injected via a field.

**Chosen approach:** inject a `headerFn func() string` field on `setupModel`. `runSetupWizard` (in `cmd/tui.go`) sets it to a closure over `model.renderBrandHeader()` — specifically a zero-value `model` with only `width` wired up. The setup view prepends `headerFn()` before its own content.

This keeps `internal/setup` free of any `lipgloss` / `tui` dependencies — the header is opaque bytes from its perspective.

---

## internal/setup API

```go
package setup

type Mode int
const (
    ModeInit     Mode = iota // dotfiles dir missing
    ModeKnotfile             // dir present, Knotfile missing
)

// Run runs the setup wizard TUI. Returns errDeclined if the user
// chose not to create a Knotfile.
func Run(dir string, mode Mode, headerFn func() string) error
```

`ErrDeclined` is a exported sentinel; callers check with `errors.Is(err, setup.ErrDeclined)`.

---

## Test Impact

`tui_test.go` stays in `package cmd` — no structural changes needed. If `internal/setup` gets its own tests later, they live in `internal/setup/setup_test.go`.

---

## Constraints

- No behaviour changes — pure file reorganisation + header addition.
- All files remain in their existing packages (`package cmd`, new `package setup`).
- `tui_test.go` must compile unchanged.
- The `max`/`min` helpers stay in `tui.go` until they can be removed (linter flags them as redundant with the built-in; leave as-is for now to avoid scope creep).
