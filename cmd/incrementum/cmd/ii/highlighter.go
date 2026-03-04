package main

import "github.com/amonks/incrementum/internal/ui"

func logHighlighter(prefixLengths map[string]int, highlight func(string, int) string) func(string) string {
	if prefixLengths == nil {
		prefixLengths = map[string]int{}
	}
	return func(id string) string {
		if id == "" {
			return id
		}
		prefixLen := ui.PrefixLength(prefixLengths, id)
		return highlight(id, prefixLen)
	}
}
