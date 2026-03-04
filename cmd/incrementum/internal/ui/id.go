package ui

import (
	"os"

	"github.com/amonks/incrementum/internal/ids"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"golang.org/x/term"
)

const (
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
	ansiReset = "\x1b[0m"
)

// HighlightID returns an ID with its unique prefix highlighted.
func HighlightID(id string, prefixLen int) string {
	if id == "" {
		return id
	}

	if prefixLen <= 0 || prefixLen > len(id) {
		return id
	}

	if !ansiEnabled() {
		return id
	}

	prefix := id[:prefixLen]
	suffix := id[prefixLen:]
	return ansiBold + ansiCyan + prefix + ansiReset + suffix
}

func ansiEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// UniqueIDPrefixLengths returns the shortest unique prefix length for each ID.
func UniqueIDPrefixLengths(inputIDs []string) map[string]int {
	return ids.UniquePrefixLengths(inputIDs)
}

// PrefixLength returns the unique prefix length for a case-insensitive ID lookup.
func PrefixLength(prefixLengths map[string]int, id string) int {
	if id == "" || prefixLengths == nil {
		return 0
	}
	return prefixLengths[internalstrings.NormalizeLower(id)]
}
