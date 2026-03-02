package creamery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"monks.co/apps/creamery/batchparser"
)

// convertParsedBatch converts a batchparser.ParsedBatch to a BatchLogEntry,
// parsing ingredient masses using the existing logic.
func convertParsedBatch(pb batchparser.ParsedBatch) (BatchLogEntry, error) {
	entry := BatchLogEntry{
		Date:         pb.Date,
		Recipe:       pb.Recipe,
		ProcessNotes: pb.ProcessNotes,
		TastingNotes: pb.TastingNotes,
	}

	for i, raw := range pb.RawIngredients {
		ing, err := parseBatchIngredient(raw.RawMass, raw.Key, i+1)
		if err != nil {
			return entry, err
		}
		entry.Ingredients = append(entry.Ingredients, ing)
	}

	return entry, nil
}

func parseBatchIngredient(rawMass, key string, line int) (BatchLogIngredient, error) {
	measurement, err := parseMassValue(rawMass)
	if err != nil {
		return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: err}
	}
	if measurement.ValueKg <= 0 {
		return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: fmt.Errorf("ingredient mass must be positive")}
	}

	return BatchLogIngredient{
		Key:              NewIngredientKey(key),
		MassKg:           measurement.ValueKg,
		RawMass:          rawMass,
		PrecisionKg:      measurement.PrecisionKg,
		PrecisionDisplay: formatPrecisionDisplay(measurement.PrecisionKg, measurement.Unit),
		Line:             line,
	}, nil
}

// ParseBatch parses a batch from the given content string.
func ParseBatch(content string) (BatchLogEntry, error) {
	pb, err := batchparser.Parse(content)
	if err != nil {
		return BatchLogEntry{}, err
	}
	return convertParsedBatch(pb)
}

// ParseBatchFile parses a batch from a file path.
func ParseBatchFile(path string) (BatchLogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BatchLogEntry{}, err
	}
	return ParseBatch(string(data))
}

// LoadBatchesFromDir loads all .batch files from a directory.
// Returns entries sorted by date, with sequence numbers assigned.
func LoadBatchesFromDir(dir string) ([]BatchLogEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var batches []BatchLogEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".batch") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		batch, err := ParseBatchFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		batches = append(batches, batch)
	}

	// Sort by date (oldest first) to assign sequence numbers
	sort.Slice(batches, func(i, j int) bool {
		return batches[i].Date.Before(batches[j].Date)
	})

	// Assign sequence numbers (1 = first-ever batch)
	for i := range batches {
		batches[i].Sequence = i + 1
	}

	// Reverse for display (newest first)
	for i, j := 0, len(batches)-1; i < j; i, j = i+1, j-1 {
		batches[i], batches[j] = batches[j], batches[i]
	}

	return batches, nil
}

// Helper functions for fda_types.go
func readDirEntries(dir string) ([]os.DirEntry, error) {
	return os.ReadDir(dir)
}

func wrapPathError(op, path string, err error) error {
	return fmt.Errorf("%s %s: %w", op, path, err)
}
