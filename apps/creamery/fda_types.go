package creamery

import (
	"path/filepath"

	"monks.co/apps/creamery/fdaparser"
)

// ParseLabel parses an FDA label from the given content string.
func ParseLabel(content string) (fdaparser.Label, error) {
	return fdaparser.Parse(content)
}

// ParseLabelFile parses an FDA label from a file path.
func ParseLabelFile(path string) (fdaparser.Label, error) {
	return fdaparser.ParseFile(path)
}

// LoadLabelsFromDir loads all .fda files from a directory.
func LoadLabelsFromDir(dir string) (map[string]fdaparser.Label, error) {
	entries, err := readDirEntries(dir)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]fdaparser.Label)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".fda" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		label, err := ParseLabelFile(path)
		if err != nil {
			return nil, wrapPathError("parse", path, err)
		}
		labels[label.ID] = label
	}
	return labels, nil
}
