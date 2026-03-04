package todo

import internalstrings "github.com/amonks/incrementum/internal/strings"

func normalizeStatus(status Status) Status {
	return Status(internalstrings.NormalizeLower(string(status)))
}

func normalizeTodoType(todoType TodoType) TodoType {
	return TodoType(internalstrings.NormalizeLower(string(todoType)))
}
