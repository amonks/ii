package todo

import internalstrings "monks.co/ii/internal/strings"

func normalizeStatus(status Status) Status {
	return Status(internalstrings.NormalizeLower(string(status)))
}

func normalizeTodoType(todoType TodoType) TodoType {
	return TodoType(internalstrings.NormalizeLower(string(todoType)))
}
