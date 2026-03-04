package job

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

func requireSnapshot(t *testing.T, name string, got string) {
	t.Helper()

	path := filepath.Join("testdata", "snapshots", name)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot missing at %s\n%s", path, got)
		}
		t.Fatalf("read snapshot %s: %v", path, err)
	}

	expected := internalstrings.TrimTrailingNewlines(internalstrings.NormalizeNewlines(string(data)))
	normalized := internalstrings.TrimTrailingNewlines(internalstrings.NormalizeNewlines(got))
	if expected != normalized {
		t.Fatalf("snapshot mismatch at %s\n--- expected ---\n%s\n--- got ---\n%s", path, expected, normalized)
	}
}
