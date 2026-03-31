" Detect files named exactly 'Knotfile' and assign the knotfile filetype.
augroup knotfile_ftdetect
  autocmd!
  autocmd BufNewFile,BufRead Knotfile set filetype=knotfile
augroup END
