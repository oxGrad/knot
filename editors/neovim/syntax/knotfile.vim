" Knotfile syntax highlighting.
" Inherits the YAML base syntax, then highlights Knotfile-specific keys.

if exists("b:current_syntax")
  finish
endif

" Load YAML base syntax so all standard YAML constructs are highlighted.
runtime! syntax/yaml.vim
unlet! b:current_syntax

syntax case match

" Top-level structural key
syntax keyword knotfileTopKey packages
  \ contained containedin=yamlBlockMappingKey,yamlFlowMappingKey

" Package field keys
syntax keyword knotfileFieldKey source target ignore condition
  \ contained containedin=yamlBlockMappingKey,yamlFlowMappingKey

" Condition sub-key
syntax keyword knotfileCondKey os
  \ contained containedin=yamlBlockMappingKey,yamlFlowMappingKey

" Known OS values for condition.os
syntax keyword knotfileOSValue darwin linux windows freebsd
  \ contained containedin=yamlPlainScalar,yamlFlowString,yamlBlockScalar

highlight default link knotfileTopKey   Structure
highlight default link knotfileFieldKey Identifier
highlight default link knotfileCondKey  Special
highlight default link knotfileOSValue  Constant

let b:current_syntax = "knotfile"
