package main

import (
	"strings"
	"unicode/utf8"
)

var trailingPunctRunes = []rune{'_', '*', '.', ',', ':', ';', '!', '?', ')', ']', '}', '"', '\'', '-'}

func normalizeTokenParts(token string) (string, string) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", ""
	}

	end := len(trimmed)
	for end > 0 {
		r, size := utf8.DecodeLastRuneInString(trimmed[:end])
		if isTrailingPunct(r) {
			end -= size
			continue
		}
		break
	}

	return trimmed[:end], trimmed[end:]
}

func normalizeToken(token string) string {
	core, _ := normalizeTokenParts(token)
	return core
}

func canonicalTagName(name string) string {
	return strings.ReplaceAll(name, "_", " ")
}

func isTrailingPunct(r rune) bool {
	for _, punct := range trailingPunctRunes {
		if r == punct {
			return true
		}
	}
	return false
}
