package batchparser

import (
	"os"
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

// Parse parses a batch from the given content string.
func Parse(content string) (ParsedBatch, error) {
	p := &batchParser{Buffer: content}
	if err := p.Init(); err != nil {
		return ParsedBatch{}, err
	}
	if err := p.Parse(); err != nil {
		return ParsedBatch{}, err
	}
	p.Execute()
	// Finish any pending note
	p.finishCurrentNote()
	return p.batch, nil
}

// ParseFile parses a batch from a file path.
func ParseFile(path string) (ParsedBatch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParsedBatch{}, err
	}
	return Parse(string(data))
}
