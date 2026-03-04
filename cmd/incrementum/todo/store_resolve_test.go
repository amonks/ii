package todo

import (
	"errors"
	"testing"
)

func TestResolveTodoIDsWithTodosExactMatch(t *testing.T) {
	todos := []Todo{{ID: "2u3iutfd"}, {ID: "abc12345"}}
	ids := []string{"2u3iutfd", "abc12345"}

	resolved, err := resolveTodoIDsWithTodos(ids, todos)
	if err != nil {
		t.Fatalf("expected resolve to succeed, got %v", err)
	}
	if len(resolved) != len(ids) {
		t.Fatalf("expected %d resolved IDs, got %d", len(ids), len(resolved))
	}
	for i, id := range ids {
		if resolved[i] != id {
			t.Fatalf("expected resolved[%d] to be %q, got %q", i, id, resolved[i])
		}
	}
}

func TestResolveTodoIDsWithTodosExactMissing(t *testing.T) {
	todos := []Todo{{ID: "2u3iutfd"}}
	_, err := resolveTodoIDsWithTodos([]string{"missing1"}, todos)
	if err == nil {
		t.Fatalf("expected missing todo error")
	}
	if !errors.Is(err, ErrTodoNotFound) {
		t.Fatalf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestResolveTodoIDsWithTodosExactUppercase(t *testing.T) {
	todos := []Todo{{ID: "2u3iutfd"}}

	resolved, err := resolveTodoIDsWithTodos([]string{"2U3IUTFD"}, todos)
	if err != nil {
		t.Fatalf("expected resolve to succeed, got %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved ID, got %d", len(resolved))
	}
	if resolved[0] != "2u3iutfd" {
		t.Fatalf("expected resolved ID to be 2u3iutfd, got %s", resolved[0])
	}
}
