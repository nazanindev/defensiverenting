package static

import "embed"

//go:embed *.css *.png authors
var Files embed.FS
