package creamery

import "embed"

// templateFiles stores all HTML templates used by the console.
//
//go:embed *.tmpl
var templateFiles embed.FS
