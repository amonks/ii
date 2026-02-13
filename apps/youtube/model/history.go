package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type HistoryEntry struct {
	Title    string
	Time     string
	TitleUrl string
}

func LoadHistory(dir string) ([]*HistoryEntry, error) {
	var history []*HistoryEntry
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		file := filepath.Join(dir, de.Name())
		var thisHistory []*HistoryEntry
		bs, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bs, &thisHistory); err != nil {
			return nil, err
		}
		history = append(history, thisHistory...)
	}
	return history, nil
}
