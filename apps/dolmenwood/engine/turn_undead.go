package engine

import (
	"strconv"
	"strings"
)

type TurnUndeadEntry struct {
	UndeadHD string
	Target   string
}

func TurnUndeadTable(class string, level int) []TurnUndeadEntry {
	if level < 1 {
		return nil
	}
	switch strings.ToLower(class) {
	case "cleric", "friar":
		// supported
	default:
		return nil
	}

	rows := turnUndeadRows()
	if level > len(rows) {
		level = len(rows)
	}
	row := rows[level-1]
	entries := make([]TurnUndeadEntry, 0, len(row))
	for i, target := range row {
		entries = append(entries, TurnUndeadEntry{
			UndeadHD: strconv.Itoa(i + 1),
			Target:   target,
		})
	}
	return entries
}

func turnUndeadRows() [][]string {
	return [][]string{
		{"7", "9", "11", "-", "-", "-", "-", "-", "-", "-", "-"},
		{"6", "8", "10", "12", "-", "-", "-", "-", "-", "-", "-"},
		{"5", "7", "9", "11", "13", "-", "-", "-", "-", "-", "-"},
		{"4", "6", "8", "10", "12", "14", "-", "-", "-", "-", "-"},
		{"3", "5", "7", "9", "11", "13", "15", "-", "-", "-", "-"},
		{"2", "4", "6", "8", "10", "12", "14", "16", "-", "-", "-"},
		{"T", "3", "5", "7", "9", "11", "13", "15", "17", "-", "-"},
		{"T", "2", "4", "6", "8", "10", "12", "14", "16", "18", "-"},
		{"T", "T", "3", "5", "7", "9", "11", "13", "15", "17", "19"},
		{"D", "T", "T", "4", "6", "8", "10", "12", "14", "16", "18"},
		{"D", "D", "T", "T", "5", "7", "9", "11", "13", "15", "17"},
		{"D", "D", "D", "T", "T", "6", "8", "10", "12", "14", "16"},
		{"D", "D", "D", "T", "T", "7", "9", "11", "13", "15", "17"},
		{"D", "D", "D", "D", "T", "T", "10", "12", "14", "16", "18"},
		{"D", "D", "D", "D", "T", "T", "11", "13", "15", "17", "19"},
	}
}
