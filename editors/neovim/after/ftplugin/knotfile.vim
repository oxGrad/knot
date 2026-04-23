" Buffer-local settings for Knotfile buffers.
" Loaded after all standard ftplugin scripts.

" Indentation — YAML convention is 2 spaces
setlocal tabstop=2
setlocal shiftwidth=2
setlocal expandtab
setlocal softtabstop=2

" YAML uses '#' for comments
setlocal commentstring=#\ %s

" Treat long lines as normal (dotfiles configs are typically short)
setlocal textwidth=0

" Tell yaml-language-server which schema to use via an inline modeline.
" Uncomment the block below to insert the modeline automatically on first open:
"
" if !search('yaml-language-server: \$schema=', 'nw')
"   call append(0, '# yaml-language-server: $schema=https://raw.githubusercontent.com/oxGrad/knot/main/schema/knotfile.schema.json')
" endif
