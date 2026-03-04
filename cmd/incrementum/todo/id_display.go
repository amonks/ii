package todo

import (
	"fmt"

	"github.com/amonks/incrementum/internal/ids"
)

// IDIndex indexes todo IDs for prefix matching and display.
type IDIndex struct {
	ids []string
}

// NewIDIndex builds an IDIndex from a slice of todos.
func NewIDIndex(todos []Todo) IDIndex {
	todoIDs := make([]string, 0, len(todos))
	for _, todo := range todos {
		todoIDs = append(todoIDs, todo.ID)
	}
	return IDIndex{ids: ids.NormalizeUniqueIDs(todoIDs)}
}

// Resolve returns the full todo ID for a prefix.
func (index IDIndex) Resolve(prefix string) (string, error) {
	if prefix == "" {
		return "", ErrTodoNotFound
	}

	match, found, ambiguous := ids.MatchPrefixNormalized(index.ids, prefix)
	if !found {
		return "", ErrTodoNotFound
	}
	if ambiguous {
		return "", fmt.Errorf("%w: %s", ErrAmbiguousTodoIDPrefix, prefix)
	}

	return match, nil
}

// PrefixLengths returns the shortest unique prefix length for each ID.
func (index IDIndex) PrefixLengths() map[string]int {
	return ids.UniquePrefixLengthsNormalized(index.ids)
}
