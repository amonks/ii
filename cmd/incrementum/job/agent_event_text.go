package job

import (
	internalstrings "monks.co/incrementum/internal/strings"
)

func formatAgentText(event agentRenderedEvent) []string {
	if event.Inline != "" {
		if event.Label != "" {
			return []string{IndentBlock(event.Label+" "+event.Inline, documentIndent)}
		}
		return []string{IndentBlock(event.Inline, documentIndent)}
	}
	labelBlank := internalstrings.IsBlank(event.Label)
	bodyBlank := internalstrings.IsBlank(event.Body)
	if labelBlank && bodyBlank {
		return nil
	}
	if bodyBlank {
		return []string{formatLogLabel(event.Label, documentIndent)}
	}
	return []string{
		formatLogLabel(event.Label, documentIndent),
		formatMarkdownBody(event.Body, subdocumentIndent),
	}
}
