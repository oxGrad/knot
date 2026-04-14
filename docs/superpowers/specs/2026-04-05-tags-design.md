# Tags Feature — Design Spec

**Date:** 2026-04-05
**Status:** Approved

---

## Overview

Add a `tags` field to Knotfile packages so that packages can be grouped into named sets. Tags enable bulk tie/untie via the CLI (`--tag work`) and a dedicated Tags tab in the interactive TUI, navigable with `[` / `]`.

---

## 1. Knotfile Syntax

Tags are an optional list of free-form strings on each package. A package may belong to zero or more tags. Packages with no `tags` field remain fully functional — they just don't appear in the Tags tab.

```yaml
packages:
  nvim:
    target: ~/.config/nvim
    tags: [work, linux]

  tmux:
    target: ~/.config/tmux
    tags: [work]

  zsh:
    target: ~/
    tags: [home]

  i3:
    target: ~/.config/i3
    tags: [linux]

  alacritty:
    target: ~/.config/alacritty
    tags: [home, linux]

  # untagged — still fully usable via package name
  secrets:
    target: ~/.ssh
```

---

## 2. Config & Data Model

### `Package` struct

Add `Tags []string` to the existing struct:

```go
type Package struct {
    Source    string     `yaml:"source"`
    Target    string     `yaml:"target"`
    Ignore    []string   `yaml:"ignore,omitempty"`
    Condition *Condition `yaml:"condition,omitempty"`
    Tags      []string   `yaml:"tags,omitempty"`
}
```

### `PackagesByTag` helper

Add to `internal/config`:

```go
// PackagesByTag returns a map of tag name → sorted slice of package names.
// Only packages that declare at least one tag are included.
func PackagesByTag(cfg *Config) map[string][]string
```

This is the single source of truth for tag→package resolution, used by both CLI and TUI.

---

## 3. CLI

### Flag

Add `--tag <name>` to `tie`, `untie`, and `plan`. Mutually exclusive with positional package args and `--all`.

```
knot tie --tag work
knot untie --tag home
knot plan --tag linux --dry-run
```

### Resolution

Extend `resolvePackageArgs` (or add a sibling `resolveTagArg`) to expand a tag name into its sorted list of package names. Returns an error if the tag name is not found in any package.

### Error cases

- `--tag` used together with package args → error: "cannot use --tag with package names"
- `--tag` used with `--all` → error: "cannot use --tag with --all"
- unknown tag name → error: `unknown tag "foo"`

---

## 4. TUI

### New tab structure

The header shows two tabs. `[` moves left, `]` moves right:

```
 Packages │ Tags          ← dim inactive, bold+underline active
```

### Model additions

```go
type tabKind int

const (
    tabPackages tabKind = iota
    tabTags
)

// added to model:
activeTab  tabKind
tagRows    []tagRow
tagCursor  int
tagOffset  int
```

### `tagRow` type

```go
type tagRow struct {
    name      string
    status    pkgStatus  // aggregate across all packages in tag
    pkgs      []pkgRow   // child package rows, in sorted order
    collapsed bool       // default: false (expanded)
}
```

### Tree rendering

The Tags tab renders tags as cyan bold headers with package children indented using tree connectors. The cursor (`▶`) can land on both tag rows and package rows.

```
▶ work                   [tied   ]
  ├── nvim               [tied   ]
  └── tmux               [tied   ]

  home                   [untied ]
  ├── zsh                [untied ]
  └── alacritty          [untied ]

  linux                  [partial] *
  ├── i3                 [tied   ]
  └── polybar            [untied ]
```

- Tag name: cyan bold, aggregate status badge
- Package children: dim, tree connectors (`├──` / `└──`), individual status badge
- Collapsed tag: children hidden, tag row shows `▶` prefix instead of nothing
- `*` suffix on tag row when any of its packages are pending

### Flattened cursor list

`visibleTagItems()` returns a flat `[]tagItem` representing what is currently rendered, respecting collapsed state. The cursor indexes into this list.

```go
type tagItem struct {
    isTag   bool
    tag     *tagRow  // set when isTag == true
    pkg     *pkgRow  // set when isTag == false
    tagName string   // parent tag name (for package items)
}
```

### Keybindings (Tags tab)

| Key | Action |
|-----|--------|
| `↑` / `k` | cursor up |
| `↓` / `j` | cursor down |
| `space` | tag row: bulk-toggle; package row: toggle that package only |
| `enter` | tag row: collapse / expand |
| `a` | apply pending changes (same as Packages tab) |
| `[` | switch to Packages tab |
| `]` | switch to Tags tab (no-op if already there) |
| `q` / `ctrl+c` | quit |

### Partial-state toggle logic (space on a tag row)

| Tag status | Space result |
|------------|-------------|
| `tied` | marks all packages for untie |
| `untied` | marks all packages for tie |
| `partial` | marks only the untied/missing packages for tie (complete the tag) |

Pressing space on a `partial` tag a second time (now `tied`) unties all. This means one press completes the tag, a second press clears it — consistent with user expectation that partial means "not done yet".

### Tag aggregate status

Computed from the union of all `LinkAction` results across every package in the tag, using the existing `computeStatus` function. A package that is `statusSkipped` (condition not met) is excluded from the non-skip count, same as individual packages.

### Reload behaviour

On `reloadMsg`, `tagRows` is rebuilt from the new config and linker state. `tagCursor` is clamped to the new visible item count. Collapsed state is **preserved** across reloads (collapsed tags stay collapsed).

---

## 5. Validation

### New checks in `knot validate`

- **Error:** tag string is empty (`tags: [""]`) → `[nvim]: tag name must not be empty`
- **Warning:** package has duplicate tags (`tags: [work, work]`) → `[nvim]: duplicate tag "work"`

No warning for untagged packages — that is valid.

### JSON Schema update

Add `tags` to the `package` definition:

```json
"tags": {
  "type": "array",
  "description": "Optional list of tag names for grouping packages. Used with --tag flag and the TUI Tags tab.",
  "items": {
    "type": "string",
    "minLength": 1
  },
  "uniqueItems": true
}
```

---

## 6. Testing

| Area | Tests to add |
|------|-------------|
| `config` | `PackagesByTag` — basic grouping, multi-tag packages, untagged packages |
| `config` | `Load` — `tags` field parsed correctly |
| `cmd/root` | `resolveTagArg` — valid tag, unknown tag, mutual-exclusion errors |
| `cmd/validate` | empty tag name → error; duplicate tag → warning |
| `cmd/tui` | `buildTagRows` — aggregate status, sorted order |
| `cmd/tui` | `visibleTagItems` — expanded/collapsed state |
| `cmd/tui` | space on tag row: tied→untie-all, untied→tie-all, partial→tie-missing |
| `cmd/tui` | `enter` on tag row: toggles collapsed |
| `cmd/tui` | `[` / `]` switches activeTab |

---

## 7. Schema

Add `tags` to the `package` definition in `schema/knotfile.schema.json`:

```json
"tags": {
  "type": "array",
  "description": "Optional list of tag names for grouping packages. Used with --tag flag and the TUI Tags tab.",
  "items": {
    "type": "string",
    "minLength": 1
  },
  "uniqueItems": true
}
```

---

## 8. Example Knotfile (`cmd/examples/Knotfile`)

Replace the existing three-package example with one that demonstrates tags:

```yaml
packages:
  nvim:
    target: ~/.config/nvim
    tags: [work]
    ignore:
      - "*.log"
      - ".DS_Store"

  tmux:
    target: ~/.config/tmux
    tags: [work]

  zsh:
    source: ./zsh
    target: ~/
    tags: [home]
    ignore:
      - "README.md"

  yabai:
    target: ~/.config/yabai
    tags: [home]
    condition:
      os: darwin

  i3:
    target: ~/.config/i3
    tags: [home, linux]
    condition:
      os: linux

  # untagged — still usable by name: knot tie secrets
  secrets:
    target: ~/.ssh
```

---

## 9. README

Three additions to `README.md`:

**1. Knotfile fields table** — add `tags` row:

| Field | Required | Description |
|---|---|---|
| `tags` | — | List of tag names; enables `--tag` flag and Tags tab in TUI |

**2. Configuration section** — update the example block to include `tags` on at least two packages (showing multi-tag and untagged cases).

**3. CLI Reference section** — document `--tag` on tie/untie/plan:

```
knot tie --tag work        Tie all packages tagged "work"
knot untie --tag home      Untie all packages tagged "home"
knot plan --tag linux      Dry-run for packages tagged "linux"
```

And add a note in the interactive TUI section:

> Use `[` / `]` to switch between the Packages tab and the Tags tab. The Tags tab shows packages grouped by tag in a tree view. Press `space` on a tag row to bulk-toggle all its packages; `enter` to collapse/expand.

---

## 10. Files Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Tags` to `Package`; add `PackagesByTag` |
| `internal/config/config_test.go` | Tests for `PackagesByTag` and tag parsing |
| `cmd/tie.go` | Add `--tag` flag; call `resolveTagArg` |
| `cmd/untie.go` | Add `--tag` flag; call `resolveTagArg` |
| `cmd/plan.go` | Add `--tag` flag |
| `cmd/root.go` | Add `resolveTagArg` helper |
| `cmd/root_test.go` | Tests for `resolveTagArg` |
| `cmd/validate.go` | Add empty/duplicate tag checks |
| `cmd/tui.go` | Tags tab: `tagRow`, `tagItem`, `visibleTagItems`, tree rendering, keybindings |
| `cmd/tui_test.go` | TUI tag logic tests |
| `schema/knotfile.schema.json` | Add `tags` property (see §7) |
| `cmd/examples/Knotfile` | Replace with tag-annotated example (see §8) |
| `README.md` | Add `tags` to field table, example, and CLI reference (see §9) |
