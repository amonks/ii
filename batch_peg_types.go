package creamery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ParsedBatch represents a batch file parsed from .batch format.
// This is the raw parsed structure before ingredient mass conversion.
type ParsedBatch struct {
	Date            time.Time
	Recipe          string
	RawIngredients  []ParsedIngredient
	ProcessNotes    []string
	TastingNotes    []string
	currentSection  string
	currentNoteLine strings.Builder
}

// ParsedIngredient is a raw ingredient line before mass parsing.
type ParsedIngredient struct {
	RawMass string
	Key     string
}

func (p *batchParser) setDate(s string) {
	date, err := time.Parse("2006-01-02", s)
	if err == nil {
		p.batch.Date = date
	}
}

func (p *batchParser) setRecipe(s string) {
	p.batch.Recipe = strings.TrimSpace(s)
}

func (p *batchParser) addIngredient(key string) {
	p.batch.RawIngredients = append(p.batch.RawIngredients, ParsedIngredient{
		RawMass: p.currentMass,
		Key:     key,
	})
}

func (p *batchParser) finishIngredient() {
	p.currentMass = ""
}

func (p *batchParser) startProcessNotes() {
	p.finishCurrentNote()
	p.batch.currentSection = "process"
}

func (p *batchParser) startTastingNotes() {
	p.finishCurrentNote()
	p.batch.currentSection = "tasting"
}

func (p *batchParser) addNoteLine(text string) {
	// Note lines starting with extra indent (4+ spaces total, so 2+ after the initial 2)
	// are continuations of the previous note.
	if len(text) > 0 && text[0] == ' ' {
		// Continuation line - append to current note
		if p.batch.currentNoteLine.Len() > 0 {
			p.batch.currentNoteLine.WriteString("\n")
		}
		p.batch.currentNoteLine.WriteString(text)
		return
	}

	// New note line - finish previous and start new
	p.finishCurrentNote()
	p.batch.currentNoteLine.WriteString(text)
}

func (p *batchParser) finishCurrentNote() {
	if p.batch.currentNoteLine.Len() == 0 {
		return
	}
	note := p.batch.currentNoteLine.String()
	p.batch.currentNoteLine.Reset()

	switch p.batch.currentSection {
	case "process":
		p.batch.ProcessNotes = append(p.batch.ProcessNotes, note)
	case "tasting":
		p.batch.TastingNotes = append(p.batch.TastingNotes, note)
	}
}

// ToBatchLogEntry converts a ParsedBatch to a BatchLogEntry,
// parsing ingredient masses using the existing logic.
func (pb ParsedBatch) ToBatchLogEntry() (BatchLogEntry, error) {
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
	p := &batchParser{Buffer: content}
	if err := p.Init(); err != nil {
		return BatchLogEntry{}, err
	}
	if err := p.Parse(); err != nil {
		return BatchLogEntry{}, err
	}
	p.Execute()
	// Finish any pending note
	p.finishCurrentNote()
	return p.batch.ToBatchLogEntry()
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

	// Sort by date
	sort.Slice(batches, func(i, j int) bool {
		return batches[i].Date.Before(batches[j].Date)
	})

	// Assign sequence numbers
	for i := range batches {
		batches[i].Sequence = i + 1
	}

	return batches, nil
}
