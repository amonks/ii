package todo

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

type noopSnapshotter struct{}

func (noopSnapshotter) Snapshot(workspacePath string) error {
	return nil
}

func newTestStore(t testing.TB) *Store {
	t.Helper()

	return &Store{
		repoPath: "test",
		wsPath:   t.TempDir(),
		snapshot: noopSnapshotter{},
		wsRelease: func() error {
			return nil
		},
	}
}

// readJSONL reads all JSON objects from a JSONL file into a slice.
func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	return readJSONLFromReader[T](f)
}

func openTestStore(t testing.TB) (*Store, error) {
	t.Helper()
	return newTestStore(t), nil
}

// readTodos reads all todos from the store.
func (s *Store) readTodos() ([]Todo, error) {
	return readJSONLStore[Todo](s, TodosFile)
}

// readDependencies reads all dependencies from the store.
func (s *Store) readDependencies() ([]Dependency, error) {
	return readJSONLStore[Dependency](s, DependenciesFile)
}

func (s *Store) getTodoByID(id string) (*Todo, error) {
	todos, err := s.readTodos()
	if err != nil {
		return nil, err
	}

	resolved, err := resolveTodoIDsWithTodos([]string{id}, todos)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, ErrTodoNotFound
	}

	for i := range todos {
		if todos[i].ID == resolved[0] {
			return &todos[i], nil
		}
	}

	return nil, ErrTodoNotFound
}

type snapshotterFunc func(string) error

func (fn snapshotterFunc) Snapshot(workspacePath string) error {
	return fn(workspacePath)
}
