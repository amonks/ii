package workflow

import (
	"path/filepath"
	"testing"
)

func TestScratchpadWriteRead(t *testing.T) {
	sp, err := newScratchpad(filepath.Join(t.TempDir(), "sp"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sp.Write("foo", "bar"); err != nil {
		t.Fatal(err)
	}
	got, err := sp.Read("foo")
	if err != nil {
		t.Fatal(err)
	}
	if got != "bar" {
		t.Errorf("expected bar, got %q", got)
	}
}

func TestScratchpadSnapshot(t *testing.T) {
	sp, err := newScratchpad(filepath.Join(t.TempDir(), "sp"))
	if err != nil {
		t.Fatal(err)
	}
	sp.Write("a", "1")
	sp.Write("b", "2")

	snap, err := sp.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap) != 2 {
		t.Errorf("expected 2 files, got %d", len(snap))
	}
	if snap["a"] != "1" || snap["b"] != "2" {
		t.Errorf("unexpected snapshot: %v", snap)
	}
}

func TestDiff(t *testing.T) {
	before := map[string]string{"a": "1", "b": "2", "c": "3"}
	after := map[string]string{"a": "1", "b": "modified", "d": "new"}

	changes := Diff(before, after)

	ops := make(map[string]ChangeOp)
	for _, c := range changes {
		ops[c.Path] = c.Op
	}
	if ops["b"] != OpModified {
		t.Errorf("expected b modified, got %v", ops["b"])
	}
	if ops["d"] != OpAdded {
		t.Errorf("expected d added, got %v", ops["d"])
	}
	if ops["c"] != OpDeleted {
		t.Errorf("expected c deleted, got %v", ops["c"])
	}
	if _, exists := ops["a"]; exists {
		t.Error("a should not appear in diff (unchanged)")
	}
}
