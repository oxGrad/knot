# 🪢 Knot

**A lightweight, configurable dotfiles manager.**

[![CI](https://github.com/oxGrad/knot/actions/workflows/ci.yml/badge.svg)](https://github.com/oxGrad/knot/actions/workflows/ci.yml)
[![Release](https://github.com/oxGrad/knot/actions/workflows/release.yml/badge.svg)](https://github.com/oxGrad/knot/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Knot is a CLI tool for managing dotfiles via symlinks. Like GNU Stow, it centralizes your configuration files in a single repository. Unlike Stow, Knot is **fully configurable** — you explicitly define where files go, ignore specific files, and apply OS-specific rules without a rigid directory structure.

## ✨ Features

- **Intuitive CLI:** Simple commands like `knot tie` and `knot untie`
- **Configurable routing:** Map any source directory to any destination on your system
- **Ignore rules:** Exclude files per package using glob patterns (e.g. `README.md`, `.DS_Store`)
- **OS conditions:** Conditionally tie packages based on the operating system
- **Safe by default:** `knot plan` previews every change before anything is written
- **Validation:** `knot validate` checks your Knotfile for errors before you run anything

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
  # Symlink the nvim config directory to ~/.config/nvim
  nvim:
    source: ./nvim
    target: ~/.config/nvim
    ignore:
      - "README.md"
      - ".DS_Store"

  # Map zsh files directly into the home directory
  zsh:
    source: ./zsh
    target: ~/

  # Only tied on macOS
  yabai:
    source: ./yabai
    target: ~/.config/yabai
    condition:
      os: darwin
```

### Knotfile fields

| Field | Required | Description |
|---|---|---|
| `source` | ✅ | Path to the source directory (relative to `Knotfile`, or absolute; `~` supported) |
| `target` | ✅ | Destination where the symlink is created (`~` supported) |
| `ignore` | — | Glob patterns matched against file basenames |
| `condition.os` | — | Only tie on this OS: `darwin`, `linux`, `windows`, `freebsd` |

## 🛠️ CLI Reference

```
knot tie [package...] [--all]    Create symlinks for one or more packages
knot untie [package...]          Remove symlinks for one or more packages
knot status                      Show current symlink state for all packages
knot plan [package...] [--all]   Dry-run: preview what tie would do
knot validate                    Validate the Knotfile for errors and warnings
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
