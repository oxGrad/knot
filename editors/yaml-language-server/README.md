# yaml-language-server Integration for Knotfile

The `Knotfile` format is YAML. The [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) (yamlls) can validate `Knotfile` documents against the official JSON Schema, providing inline error highlighting, hover documentation, and auto-completions for all fields.

**Schema URL:**
```
https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json
```

---

## Method 1 — Inline modeline (universal, works in every editor)

Add this comment as the **first line** of any `Knotfile`. `yaml-language-server` detects the `$schema` modeline automatically, regardless of editor configuration:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json
packages:
  nvim:
    source: ./nvim
    target: ~/.config/nvim
    ignore:
      - "README.md"
      - ".DS_Store"
  zsh:
    source: ./zsh
    target: ~/
```

This is the simplest approach and requires no editor-side setup.

---

## Method 2 — Neovim (nvim-lspconfig)

Add the schema mapping to your `yamlls` lspconfig setup:

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

If you use the `knot.nvim` plugin (see `../neovim/README.md`) with `auto_configure_yamlls = true`, this is handled automatically at runtime without any manual lspconfig changes.

---

## Method 3 — VS Code

Copy the contents of `settings.json` (in this directory) into your workspace's `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json": "**/Knotfile"
  },
  "yaml.validate": true,
  "yaml.completion": true,
  "yaml.hover": true
}
```

Requires the [YAML extension by Red Hat](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml).

---

## Method 4 — Global yamlls settings

Some editors expose yamlls settings globally. Add the schema association to whichever configuration file your editor reads for `yaml-language-server`:

```json
{
  "schemas": {
    "https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json": ["**/Knotfile", "Knotfile"]
  }
}
```

---

## What the schema validates

| Field | Rule |
|---|---|
| `packages` | Required top-level key; each value is a package definition |
| `source` | Required string; relative paths resolved from `Knotfile` directory |
| `target` | Required string; destination for symlinks |
| `ignore` | Optional array of glob strings (unique items) |
| `condition.os` | Optional; must be one of `darwin`, `linux`, `windows`, `freebsd` |

Unknown top-level keys and unknown package fields are flagged as errors (`additionalProperties: false`).
