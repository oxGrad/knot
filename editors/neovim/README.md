# knot.nvim

Neovim plugin for the [knot](https://github.com/oxgrad/knot) dotfiles manager.

## Features

- Automatic filetype detection for files named exactly `Knotfile`
- 🪢 Devicon registration for [nvim-web-devicons](https://github.com/nvim-tree/nvim-web-devicons) (shown in statuslines, file trees, tablines)
- YAML-based syntax highlighting with Knotfile-specific keyword groups:
  - `packages` — highlighted as a structure keyword
  - `source`, `target`, `ignore`, `condition` — highlighted as identifiers
  - `os` — highlighted as a special keyword
  - `darwin`, `linux`, `windows`, `freebsd` — highlighted as constants
- Buffer-local settings (`tabstop=2`, `shiftwidth=2`, `commentstring=# %s`)
- Optional `yaml-language-server` schema auto-configuration for inline validation and completions
- Treesitter YAML parser override for enhanced syntax and text-objects (Neovim 0.9+)

## Requirements

- Neovim 0.8+
- **Optional:** [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig) + [`yaml-language-server`](https://github.com/redhat-developer/yaml-language-server) for schema validation and completions
- **Optional:** [nvim-treesitter](https://github.com/nvim-treesitter/nvim-treesitter) with `yaml` parser (Neovim 0.9+) for enhanced highlighting

## Installation

### lazy.nvim

```lua
{
  -- Local path (from inside the knot repo):
  dir = vim.fn.expand("~/path/to/knot/editors/neovim"),
  -- Or, once published as a standalone plugin:
  -- "oxgrad/knot.nvim",
  ft = "knotfile",
  opts = {
    auto_configure_yamlls = true,
  },
},
```

### packer.nvim

```lua
use {
  "~/path/to/knot/editors/neovim",
  -- Or: "oxgrad/knot.nvim",
  config = function()
    require("knot").setup({
      auto_configure_yamlls = true,
    })
  end,
}
```

### Manual (no plugin manager)

Add the plugin directory to your runtime path in `init.lua`:

```lua
vim.opt.runtimepath:append(vim.fn.expand("~/path/to/knot/editors/neovim"))
require("knot").setup()
```

Or in `init.vim`:

```vim
set runtimepath+=~/path/to/knot/editors/neovim
lua require("knot").setup()
```

## Configuration

```lua
require("knot").setup({
  -- Set to false if you manage yamlls schemas yourself.
  auto_configure_yamlls = true,
})
```

| Option | Type | Default | Description |
|---|---|---|---|
| `auto_configure_yamlls` | `boolean` | `true` | Automatically notify the active `yaml-language-server` client to use the Knotfile JSON Schema for all `Knotfile` buffers. |

## Manual yaml-language-server schema association

If you prefer to configure `yamlls` yourself, add this to your `lspconfig` setup:

```lua
require("lspconfig").yamlls.setup({
  settings = {
    yaml = {
      schemas = {
        ["https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json"] = "**/Knotfile",
      },
    },
  },
})
```

Or add this modeline as the first line of any `Knotfile`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json
packages:
  nvim:
    source: ./nvim
    target: ~/.config/nvim
```

## Schema

The official JSON Schema is published at:

```
https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json
```

See [`../../schema/knotfile.schema.json`](../../schema/knotfile.schema.json) for the full definition.
