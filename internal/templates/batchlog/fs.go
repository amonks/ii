package batchlogtemplates

import "embed"

// Files exposes the compiled batch log templates.
//
//go:embed *.tmpl
var Files embed.FS
