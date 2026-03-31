--- knot.nvim — Neovim support for the knot dotfiles manager.
---
--- Provides:
---   - Filetype detection for files named exactly "Knotfile"
---   - Treesitter YAML parser override for knotfile buffers
---   - Optional automatic yaml-language-server schema configuration
---
--- Minimum requirement: Neovim 0.8+

local M = {}

--- The published JSON Schema URL for Knotfile validation.
M.schema_url = "https://raw.githubusercontent.com/oxgrad/knot/main/schema/knotfile.schema.json"

local defaults = {
  -- Automatically tell yaml-language-server to validate Knotfile buffers
  -- using the official JSON Schema. Requires nvim-lspconfig + yamlls.
  auto_configure_yamlls = true,
}

--- Set up the plugin.
---@param opts table|nil  Optional overrides for the default config table.
function M.setup(opts)
  local cfg = vim.tbl_deep_extend("force", defaults, opts or {})

  -- 1. Register "Knotfile" → "knotfile" filetype mapping.
  --    Belt-and-suspenders alongside ftdetect/knotfile.vim.
  vim.filetype.add({
    filename = {
      ["Knotfile"] = "knotfile",
    },
  })

  -- 2. Override treesitter parser to "yaml" for knotfile buffers so that
  --    tree-sitter-yaml highlighting and text-objects work out of the box.
  vim.api.nvim_create_autocmd("FileType", {
    pattern = "knotfile",
    group = vim.api.nvim_create_augroup("KnotfileTreesitter", { clear = true }),
    callback = function(ev)
      -- vim.treesitter.start() requires Neovim 0.9+
      if vim.fn.has("nvim-0.9") == 1 then
        local ok, parsers = pcall(require, "nvim-treesitter.parsers")
        if ok and parsers.has_parser("yaml") then
          vim.treesitter.start(ev.buf, "yaml")
        end
      end
    end,
  })

  -- 3. Optionally configure yamlls schema at runtime.
  if cfg.auto_configure_yamlls then
    M._configure_yamlls()
  end
end

--- Notify the active yaml-language-server client to associate the Knotfile
--- schema with all files named "Knotfile" via workspace/didChangeConfiguration.
function M._configure_yamlls()
  vim.api.nvim_create_autocmd("FileType", {
    pattern = "knotfile",
    group = vim.api.nvim_create_augroup("KnotfileYamlls", { clear = true }),
    callback = function()
      local clients = vim.lsp.get_clients({ name = "yamlls" })
      for _, client in ipairs(clients) do
        local settings = vim.tbl_deep_extend("force", client.config.settings or {}, {
          yaml = {
            schemas = {
              [M.schema_url] = "**/Knotfile",
            },
          },
        })
        client.notify("workspace/didChangeConfiguration", { settings = settings })
      end
    end,
  })
end

return M
