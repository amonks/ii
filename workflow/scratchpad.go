package workflow

import (
	"fmt"
	"os"
	"path/filepath"
)

// scratchpad manages the persistent scratchpad directory for a workflow execution.
type scratchpad struct {
	dir string
}

func newScratchpad(dir string) (*scratchpad, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create scratchpad: %w", err)
	}
	return &scratchpad{dir: dir}, nil
}

// Dir returns the absolute path to the scratchpad directory.
func (s *scratchpad) Dir() string {
	return s.dir
}

// Write writes content to a named file in the scratchpad.
func (s *scratchpad) Write(name, content string) error {
	return os.WriteFile(filepath.Join(s.dir, name), []byte(content), 0o644)
}

// Read reads a named file from the scratchpad.
func (s *scratchpad) Read(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Snapshot captures the current state of all files in the scratchpad.
func (s *scratchpad) Snapshot() (map[string]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	snap := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		snap[e.Name()] = string(data)
	}
	return snap, nil
}

// Diff compares two snapshots and returns the changes.
func Diff(before, after map[string]string) []ScratchpadChange {
	var changes []ScratchpadChange
	for name, content := range after {
		prev, existed := before[name]
		if !existed {
			changes = append(changes, ScratchpadChange{Path: name, Op: OpAdded, Content: content})
		} else if content != prev {
			changes = append(changes, ScratchpadChange{Path: name, Op: OpModified, Content: content})
		}
	}
	for name := range before {
		if _, exists := after[name]; !exists {
			changes = append(changes, ScratchpadChange{Path: name, Op: OpDeleted})
		}
	}
	return changes
}

// Remove deletes the scratchpad directory.
func (s *scratchpad) Remove() error {
	return os.RemoveAll(s.dir)
}

// ScratchpadChange records a single file change.
type ScratchpadChange struct {
	Path    string
	Op      ChangeOp
	Content string // Empty for deleted files.
}

// ChangeOp is the type of scratchpad file change.
type ChangeOp string

const (
	OpAdded    ChangeOp = "added"
	OpModified ChangeOp = "modified"
	OpDeleted  ChangeOp = "deleted"
)
