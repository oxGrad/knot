# 🪢 Knot

**A lightweight, configurable dotfiles manager.**

[![Go Report Card](https://goreportcard.com/badge/github.com/oxGrad/knot)](https://goreportcard.com/report/github.com/oxGrad/knot)
[![CI](https://github.com/oxGrad/knot/actions/workflows/ci.yml/badge.svg)](https://github.com/oxGrad/knot/actions/workflows/ci.yml)
[![Release](https://github.com/oxGrad/knot/actions/workflows/release.yml/badge.svg)](https://github.com/oxGrad/knot/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**Knot** is a CLI tool for managing your dotfiles. Like GNU Stow, it relies on symlinks to keep your configuration files centralized in a single repository. Unlike Stow, Knot is **fully configurable** — you explicitly define where files go, ignore specific files, and apply OS-specific rules without being forced into a rigid directory structure.

## ✨ Features

- **Intuitive CLI:** Simple commands like `knot tie` and `knot untie`
- **Configurable Routing:** Map any file in your dotfiles repo to any location on your system
- **Ignore Rules:** Exclude specific files (e.g. `README.md`, `.DS_Store`) per package
- **OS Conditions:** Conditionally tie packages based on the operating system (macOS vs Linux)
- **Safe by Default:** `knot plan` previews every change before anything is written
- **Validation:** `knot validate` checks your Knotfile for errors before you run anything

## 🚀 Installation

### Homebrew (recommended)

```bash
brew install oxGrad/tap/knot
```

This installs a pre-built binary for macOS (Intel + Apple Silicon) and Linux via the
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

Create a file named exactly `Knotfile` (no extension) at the root of your dotfiles repository. Knot searches upward from the current directory to find it automatically.

```yaml
packages:
  # A simple 1-to-1 mapping
  nvim:
    target: ~/.config/nvim
    source: ./nvim
    ignore:
      - "README.md"
      - ".DS_Store"

  # Map files directly to the home directory
  zsh:
    target: ~/
    source: ./zsh

  # OS-specific package — only tied on macOS
  yabai:
    target: ~/.config/yabai
    source: ./yabai
    condition:
      os: darwin
```

### Knotfile fields

| Field | Required | Description |
|---|---|---|
| `source` | ✅ | Path to source directory (relative to `Knotfile`, or absolute; `~` supported) |
| `target` | ✅ | Destination directory where symlinks are created (`~` supported) |
| `ignore` | — | List of glob patterns matched against file basenames |
| `condition.os` | — | Only tie on this OS (`darwin`, `linux`, `windows`, `freebsd`) |

A [JSON Schema](schema/knotfile.schema.json) is available for editor validation and auto-complete — see [Editor Integration](#editor-integration).

## 🛠️ CLI Reference

```
knot tie [package...] [--all]   Create symlinks for one or more packages
knot untie [package...]          Remove symlinks for one or more packages
knot status                      Show current symlink state for all packages
knot plan [package...] [--all]  Dry-run: preview what tie would do
knot validate                    Validate the Knotfile for errors and warnings
```

Global flags available on every command:

```
--config string   Path to Knotfile (default: auto-discover upward from cwd)
--dry-run         Print actions without executing them
```

### `knot tie`

Creates symlinks for the specified packages. Skips files that are already correctly linked.
Warns on conflicts (target exists but is not the expected symlink) without overwriting.

```bash
knot tie nvim zsh        # tie specific packages
knot tie --all           # tie every package in the Knotfile
knot tie nvim --dry-run  # preview without writing
```

### `knot untie`

Removes symlinks previously created by `knot tie`.

```bash
knot untie nvim
```

### `knot status`

Shows the current state of every managed symlink:

```
[OK]       ~/.config/nvim/init.lua
[MISSING]  ~/.zshrc
[CONFLICT] ~/.config/nvim/lazy-lock.json: target exists and is not a symlink
```

### `knot plan`

Dry-run that shows exactly what `tie` would do, with a summary line:

```
  + ~/.config/nvim/init.lua -> /dotfiles/nvim/init.lua
  = ~/.config/nvim/options.lua (already linked)

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

## 🖥️ Editor Integration

### Neovim

A full Neovim plugin lives in [`editors/neovim/`](editors/neovim/). It provides filetype detection,
syntax highlighting, Treesitter YAML override, 🪢 devicon, and automatic `yaml-language-server`
schema configuration. See [`editors/neovim/README.md`](editors/neovim/README.md) for installation
instructions (lazy.nvim, packer.nvim, and manual).

### YAML Language Server (VS Code and others)

Add the Knotfile schema to your editor's YAML LS settings:

```json
{
  "yaml.schemas": {
    "https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json": "**/Knotfile"
  }
}
```

Or use an inline modeline as the first line of any `Knotfile`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json
packages:
  ...
```

See [`editors/yaml-language-server/README.md`](editors/yaml-language-server/README.md) for all
integration methods.

## 📦 Releasing a new version

Releases are fully automated via [GoReleaser](https://goreleaser.com). Push a semver tag to `main`:

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
