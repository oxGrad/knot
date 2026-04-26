# 🪢 Knot

**A lightweight, configurable dotfiles manager.**

[![Go Report Card](https://goreportcard.com/badge/github.com/oxGrad/knot)](https://goreportcard.com/report/github.com/oxGrad/knot)
[![CI](https://github.com/oxGrad/knot/actions/workflows/ci.yml/badge.svg)](https://github.com/oxGrad/knot/actions/workflows/ci.yml)
[![Release](https://github.com/oxGrad/knot/actions/workflows/release.yml/badge.svg)](https://github.com/oxGrad/knot/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Knot is a CLI tool for managing dotfiles via symlinks. Like GNU Stow, it centralizes your configuration files in a single repository. Unlike Stow, Knot is **fully configurable** — you explicitly define where files go, ignore specific files, and apply OS-specific rules without a rigid directory structure.

## ✨ Features

- **Intuitive CLI:** Simple commands like `knot tie` and `knot untie`
- **Configurable Routing:** Map any file in your dotfiles repo to any location on your system
- **Ignore Rules:** Exclude specific files (e.g. `README.md`, `.DS_Store`) per package
- **OS Conditions:** Conditionally tie packages based on the operating system (macOS vs Linux)
- **Tags:** Group packages with `tags: [work, linux]` and bulk-operate with `--tag <name>`
- **Safe by Default:** `knot plan` previews every change before anything is written
- **Validation:** `knot validate` checks your Knotfile for errors before you run anything
- **Interactive TUI:** Run `knot` with no arguments for a live package/tags dashboard

## 🚀 Installation

### Homebrew (recommended)

```bash
brew install oxGrad/tap/knot
```

Installs a pre-built binary for macOS (Intel + Apple Silicon) and Linux via the
[oxGrad/homebrew-tap](https://github.com/oxGrad/homebrew-tap) tap.

### Download a release binary

Pre-built binaries for Linux, macOS, and Windows are available on the
[Releases](https://github.com/oxGrad/knot/releases) page.

```bash
# Example for Linux amd64
curl -L https://github.com/oxGrad/knot/releases/latest/download/knot_linux_amd64.tar.gz | tar xz
sudo mv knot /usr/local/bin/
```

### Go install

```bash
go install github.com/oxgrad/knot@latest
```

## ⚙️ Configuration (`Knotfile`)

Create a file named exactly `Knotfile` (no extension) at the root of your dotfiles repository.
Knot searches upward from the current directory to find it automatically.

```yaml
packages:
  # source defaults to ./nvim when omitted
  nvim:
    target: ~/.config/nvim
    tags: [work]
    ignore:
      - "README.md"
      - ".DS_Store"

  # Map zsh files directly into the home directory
  zsh:
    source: ./zsh
    tags: [home]

  # OS-specific package — only tied on macOS; belongs to two tags
  yabai:
    target: ~/.config/yabai
    tags: [home, macos]
    condition:
      os: darwin

  # Untagged — still usable by name
  secrets:
    target: ~/.ssh
```

### Knotfile fields

| Field | Required | Description |
|---|---|---|
| `source` | — | Path to source directory (relative to `Knotfile`, or absolute; `~` supported). Defaults to `./<package-name>` |
| `target` | ✅ | Destination path for symlinks (`~` supported). See linking modes below. |
| `ignore` | — | List of glob patterns matched against file basenames |
| `tags` | — | List of tag names; enables `--tag` flag and Tags tab in TUI |
| `condition.os` | — | Only tie on this OS (`darwin`, `linux`, `windows`, `freebsd`) |

### Linking modes

The `target` value controls how knot places symlinks:

**Directory symlink** (default) — knot creates a single symlink at `target` pointing to the entire source directory. Use this when the target path does not yet exist and you want the whole directory to be managed as one unit.

```yaml
nvim:
  target: ~/.config/nvim   # creates ~/.config/nvim -> /dotfiles/nvim
```

**Per-file mode** — add a trailing `/` to `target`. Knot links each file in the source directory individually into `target`. This is required when `target` is a directory that must already exist (like `~/` or `~/.config/`), and also respects `ignore` patterns.

```yaml
zsh:
  target: ~/               # links ~/dotfiles/zsh/.zshrc -> ~/.zshrc, etc.
  ignore:
    - "README.md"
```

A [JSON Schema](schema/knotfile.schema.json) is available for editor validation and auto-complete — see [Editor Integration](#editor-integration).

## 🛠️ CLI Reference

```
knot tie [package...] [--all] [--tag <name>]   Create symlinks
knot untie [package...] [--all] [--tag <name>]  Remove symlinks
knot status                                       Show symlink state
knot plan [package...] [--all] [--tag <name>]   Dry-run preview
knot validate                                     Validate Knotfile
```

Global flags available on every command:

```
--config string   Path to Knotfile (default: auto-discover upward from cwd)
--dry-run         Print actions without executing them
```

### `knot tie`

Creates symlinks for the specified packages. Skips packages that are already correctly linked.
Warns on conflicts (target exists but is not the expected symlink) without overwriting.

```bash
knot tie nvim zsh        # tie specific packages
knot tie --all           # tie every package in the Knotfile
knot tie --tag work      # tie all packages tagged "work"
knot tie nvim --dry-run  # preview without writing
```

### `knot untie`

Removes symlinks previously created by `knot tie`.

```bash
knot untie nvim
knot untie --tag home    # untie all packages tagged "home"
```

### `knot status`

Shows the current state of every managed symlink:

```
[OK]       ~/.config/nvim
[MISSING]  ~/.zshrc
[CONFLICT] ~/.config/karabiner: target exists and is not a symlink
```

### `knot plan`

Dry-run that shows exactly what `tie` would do:

```
  + ~/.config/nvim -> /dotfiles/nvim
  = ~/.zshrc (already linked)

Plan: 1 to create, 0 to remove, 1 already linked, 0 conflicts
```

### `knot validate`

Validates the Knotfile without touching the filesystem:

```bash
knot validate
# Validating Knotfile: /home/user/dotfiles/Knotfile
#
#   ERROR [yabai]: source directory "/home/user/dotfiles/yabai" does not exist
#
# Validation failed: 1 error(s), 0 warning(s)
```

Exit codes: `0` = valid · `1` = errors · `2` = warnings only

### Interactive TUI

Run `knot` with no arguments to launch the interactive TUI. It shows a live view of all packages and lets you toggle, apply, and reload without typing individual commands.

The TUI has two tabs: **Packages** (the default) and **Tags**. Switch between them with `[` and `]`. The Tags tab shows packages grouped by tag in a collapsible tree view — press `enter` to collapse or expand a tag, and `space` to bulk-toggle all packages in a tag.

Key bindings:

| Key | Action |
|---|---|
| `↑`/`↓` or `j`/`k` | Navigate |
| `space` | Toggle package / bulk-toggle tag |
| `enter` | Collapse/expand tag (Tags tab) |
| `[` / `]` | Switch tabs |
| `a` | Apply pending changes |
| `r` | `git pull` and reload |
| `b` | Switch branch |
| `e` | Open dotfiles dir in `$EDITOR` |
| `q` | Quit |

## 🖥️ Editor Integration

Knot ships a [JSON Schema](schema/knotfile.schema.json) for the Knotfile format, enabling inline
validation, hover documentation, and auto-completions in any editor that supports
[yaml-language-server](https://github.com/redhat-developer/yaml-language-server).

**Schema URL:**
```
https://raw.githubusercontent.com/oxGrad/knot/main/schema/knotfile.schema.json
```

### Inline modeline (any editor)

Add this comment as the **first line** of any `Knotfile`. yaml-language-server picks it up
automatically regardless of which editor you use — no editor configuration required.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oxGrad/knot/main/schema/knotfile.schema.json
packages:
  nvim:
    source: ./nvim
    target: ~/.config/nvim
```

### VS Code

Copy into your workspace `.vscode/settings.json`. Requires the
[YAML extension by Red Hat](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml).

```json
{
  "yaml.schemas": {
    "https://raw.githubusercontent.com/oxGrad/knot/main/schema/knotfile.schema.json": "**/Knotfile"
  },
  "yaml.validate": true,
  "yaml.completion": true,
  "yaml.hover": true
}
```

### nvim-lspconfig

Add the schema to your `yamlls` setup:

```lua
require("lspconfig").yamlls.setup({
  settings = {
    yaml = {
      schemas = {
        ["https://raw.githubusercontent.com/oxGrad/knot/main/schema/knotfile.schema.json"] = "**/Knotfile",
      },
    },
  },
})
```

### Global yamlls config (Helix, Zed, and others)

Add the schema to whichever config file your editor reads for yaml-language-server:

```json
{
  "schemas": {
    "https://raw.githubusercontent.com/oxGrad/knot/main/schema/knotfile.schema.json": ["**/Knotfile", "Knotfile"]
  }
}
```

### Neovim plugin

A full Neovim plugin lives in [`editors/neovim/`](editors/neovim/). It provides:

- Filetype detection for files named `Knotfile`
- YAML syntax highlighting with Knotfile-specific keyword groups
- Treesitter YAML parser override (Neovim 0.9+)
- 🪢 devicon registration for nvim-web-devicons
- Automatic yaml-language-server schema configuration at runtime (no manual lspconfig setup needed)

See [`editors/neovim/README.md`](editors/neovim/README.md) for installation instructions
(lazy.nvim, packer.nvim, and manual).

## 📦 Releasing a new version

Releases are automated via [GoReleaser](https://goreleaser.com). Push a semver tag to `main`:

```bash
git tag v1.2.3
git push origin v1.2.3
```

This triggers the release workflow which:
1. Builds binaries for Linux, macOS (Intel + Apple Silicon), and Windows
2. Creates a GitHub Release with archives and `checksums.txt`
3. Pushes an updated `knot.rb` formula to [oxGrad/homebrew-tap](https://github.com/oxGrad/homebrew-tap)

> **Prerequisite:** A `HOMEBREW_TAP_GITHUB_TOKEN` repository secret must be set — a GitHub PAT
> with `contents: write` permission on the `oxGrad/homebrew-tap` repository.

## 🤝 Contributing

Pull requests are welcome. The CI pipeline runs on every PR to `main`:

| Check | Tool |
|---|---|
| Tests | `go test ./...` |
| Build | `go build ./...` |
| Lint | `golangci-lint` |

All three checks must pass before a PR can be merged.

## License

[MIT](https://opensource.org/licenses/MIT)
