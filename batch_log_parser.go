package creamery

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type rawLine struct {
	Text string
	Line int
}

// ParseBatchLog reads the batch log format and returns the ordered entries.
func ParseBatchLog(r io.Reader) ([]BatchLogEntry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var lines []rawLine
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		lines = append(lines, rawLine{
			Text: scanner.Text(),
			Line: lineNo,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	records, err := buildBatchRecords(lines)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	entries := make([]BatchLogEntry, 0, len(records))
	for _, rec := range records {
		entry, err := rec.BuildEntry()
		if err != nil {
			if parseErr, ok := err.(BatchLogParseError); ok {
				return nil, parseErr
			}
			return nil, BatchLogParseError{Line: rec.StartLine, Err: err}
		}
		entry.Sequence = len(entries) + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

func buildBatchRecords(lines []rawLine) ([]rawBatchRecord, error) {
	var records []rawBatchRecord
	current := rawBatchRecord{}

	flushRecord := func() {
		if len(current.Fields) == 0 {
			return
		}
		records = append(records, current)
		current = rawBatchRecord{}
	}

	for i := 0; i < len(lines); {
		raw := lines[i]
		text := raw.Text
		trimmed := strings.TrimSpace(text)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "%%") {
			flushRecord()
			i++
			continue
		}
		if leadingSpaces(text) > 0 {
			return nil, BatchLogParseError{Line: raw.Line, Err: errors.New("unexpected indentation")}
		}
		key, value, err := parseBatchLogField(trimmed)
		if err != nil {
			return nil, BatchLogParseError{Line: raw.Line, Err: err}
		}
		if current.StartLine == 0 {
			current.StartLine = raw.Line
		}

		// Block value
		if value == "" {
			block, nextIdx, blockErr := collectBlockLines(lines, i+1, raw)
			if blockErr != nil {
				return nil, blockErr
			}
			current.Fields = append(current.Fields, rawBatchField{
				Key:   key,
				Line:  raw.Line,
				Block: block,
			})
			i = nextIdx
			continue
		}

		current.Fields = append(current.Fields, rawBatchField{
			Key:   key,
			Value: value,
			Line:  raw.Line,
		})
		i++
	}

	flushRecord()
	return records, nil
}

type rawBatchRecord struct {
	StartLine int
	Fields    []rawBatchField
}

type rawBatchField struct {
	Key   string
	Value string
	Line  int
	Block []rawBlockLine
}

type rawBlockLine struct {
	Line int
	Text string
}

func collectBlockLines(lines []rawLine, start int, keyLine rawLine) ([]rawBlockLine, int, error) {
	block := make([]rawBlockLine, 0)
	baseIndent := -1
	i := start

	for i < len(lines) {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw.Text)
		if trimmed == "" {
			block = append(block, rawBlockLine{Line: raw.Line, Text: ""})
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		if strings.HasPrefix(trimmed, "%%") {
			break
		}
		indent := leadingSpaces(raw.Text)
		if indent == 0 {
			break
		}
		if baseIndent == -1 {
			baseIndent = indent
		}
		if indent < baseIndent {
			break
		}
		if baseIndent > len(raw.Text) {
			baseIndent = len(raw.Text)
		}
		content := raw.Text[baseIndent:]
		block = append(block, rawBlockLine{
			Line: raw.Line,
			Text: content,
		})
		i++
	}

	block = trimEmptyEdges(block)
	if len(block) == 0 {
		return nil, i, BatchLogParseError{Line: keyLine.Line, Err: errors.New("expected indented block after field")}
	}
	return block, i, nil
}

func trimEmptyEdges(lines []rawBlockLine) []rawBlockLine {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start].Text) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1].Text) == "" {
		end--
	}
	return lines[start:end]
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
			continue
		}
		if r == '\t' {
			count++
			continue
		}
		break
	}
	return count
}

func parseBatchLogField(line string) (string, string, error) {
	idx := strings.IndexRune(line, ':')
	if idx <= 0 {
		return "", "", fmt.Errorf("expected key: value, got %q", line)
	}
	key := strings.ToLower(strings.TrimSpace(line[:idx]))
	value := strings.TrimSpace(line[idx+1:])
	return key, value, nil
}

func (r rawBatchRecord) BuildEntry() (BatchLogEntry, error) {
	entry := BatchLogEntry{}

	for _, field := range r.Fields {
		switch field.Key {
		case "date":
			date, err := time.Parse("2006-01-02", strings.TrimSpace(field.Value))
			if err != nil {
				return entry, BatchLogParseError{Line: field.Line, Err: fmt.Errorf("invalid date %q", field.Value)}
			}
			entry.Date = date
		case "recipe":
			entry.Recipe = strings.TrimSpace(field.Value)
		case "ingredient":
			ing, err := parseIngredientField(field.Value, field.Line)
			if err != nil {
				return entry, err
			}
			entry.Ingredients = append(entry.Ingredients, ing)
		case "ingredients":
			if len(field.Block) > 0 {
				if err := addIngredientsFromBlock(&entry, field.Block); err != nil {
					return entry, err
				}
				break
			}
			ing, err := parseIngredientField(field.Value, field.Line)
			if err != nil {
				return entry, err
			}
			entry.Ingredients = append(entry.Ingredients, ing)
		case "process", "process_notes":
			if len(field.Block) > 0 {
				entry.ProcessNotes = append(entry.ProcessNotes, blockLinesToParagraphs(field.Block)...)
				break
			}
			entry.ProcessNotes = append(entry.ProcessNotes, strings.TrimSpace(field.Value))
		case "tasting", "tasting_notes":
			if len(field.Block) > 0 {
				entry.TastingNotes = append(entry.TastingNotes, blockLinesToParagraphs(field.Block)...)
				break
			}
			entry.TastingNotes = append(entry.TastingNotes, strings.TrimSpace(field.Value))
		default:
			return entry, BatchLogParseError{Line: field.Line, Err: fmt.Errorf("unsupported field %q", field.Key)}
		}
	}

	if len(entry.Ingredients) == 0 {
		return entry, BatchLogParseError{Line: r.StartLine, Err: errors.New("batch is missing ingredient weights")}
	}

	return entry, nil
}

func parseIngredientField(value string, line int) (BatchLogIngredient, error) {
	clean := strings.TrimSpace(value)
	if idx := strings.Index(clean, "#"); idx >= 0 {
		clean = strings.TrimSpace(clean[:idx])
	}
	if clean == "" {
		return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: errors.New("ingredient entry is empty")}
	}

	var keyPart, massPart string
	if strings.Contains(clean, "=") {
		parts := strings.SplitN(clean, "=", 2)
		keyPart = strings.TrimSpace(parts[0])
		massPart = strings.TrimSpace(parts[1])
	} else {
		fields := strings.Fields(clean)
		if len(fields) < 2 {
			return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: errors.New("ingredient must include a key and mass")}
		}
		if candidate := strings.Join(fields[1:], " "); candidate != "" {
			if _, err := parseMassValue(candidate); err == nil {
				keyPart = fields[0]
				massPart = candidate
			}
		}
		if keyPart == "" {
			var used int
			var candidate string
			for i := 1; i < len(fields); i++ {
				part := strings.Join(fields[:i], " ")
				if _, err := parseMassValue(part); err == nil {
					used = i
					candidate = part
				}
			}
			if used == 0 {
				return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: errors.New("unable to parse ingredient mass")}
			}
			massPart = candidate
			keyPart = strings.TrimSpace(strings.Join(fields[used:], " "))
		}
	}

	if keyPart == "" {
		return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: errors.New("ingredient key is empty")}
	}
	mass, err := parseMassValue(massPart)
	if err != nil {
		return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: err}
	}
	if mass <= 0 {
		return BatchLogIngredient{}, BatchLogParseError{Line: line, Err: errors.New("ingredient mass must be positive")}
	}

	return BatchLogIngredient{
		Key:     NewIngredientKey(keyPart),
		MassKg:  mass,
		RawMass: massPart,
		Line:    line,
	}, nil
}

func addIngredientsFromBlock(entry *BatchLogEntry, block []rawBlockLine) error {
	for _, line := range block {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "-") {
			text = strings.TrimSpace(strings.TrimPrefix(text, "-"))
		}
		if text == "" {
			continue
		}
		ing, err := parseIngredientField(text, line.Line)
		if err != nil {
			return err
		}
		entry.Ingredients = append(entry.Ingredients, ing)
	}
	return nil
}

func parseMassValue(input string) (float64, error) {
	clean := strings.TrimSpace(strings.ToLower(input))
	if idx := strings.Index(clean, "#"); idx >= 0 {
		clean = strings.TrimSpace(clean[:idx])
	}
	if clean == "" {
		return 0, errors.New("missing mass value")
	}

	fields := strings.Fields(clean)
	switch len(fields) {
	case 0:
		return 0, errors.New("missing mass value")
	case 1:
		return parseCompactMass(fields[0])
	default:
		numberToken := fields[0]
		unitToken := strings.Join(fields[1:], "")
		value, err := strconv.ParseFloat(numberToken, 64)
		if err != nil {
			return parseCompactMass(numberToken + unitToken)
		}
		return applyMassUnit(value, unitToken)
	}
}

func parseCompactMass(token string) (float64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, errors.New("missing mass value")
	}
	numPart, unitPart := splitNumberAndUnit(token)
	if numPart == "" {
		return 0, fmt.Errorf("invalid mass %q", token)
	}
	value, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid mass %q", token)
	}
	return applyMassUnit(value, unitPart)
}

func applyMassUnit(value float64, unit string) (float64, error) {
	if value <= 0 {
		return 0, errors.New("mass must be positive")
	}
	unit = strings.Trim(unit, ". ")
	if unit == "" {
		return value, nil
	}
	switch unit {
	case "kg", "kilogram", "kilograms":
		return value, nil
	case "g", "gram", "grams":
		return value / 1000, nil
	case "lb", "lbs", "pound", "pounds":
		return value * 0.45359237, nil
	case "oz", "ounce", "ounces":
		return value * 0.028349523125, nil
	default:
		return 0, fmt.Errorf("unknown mass unit %q", unit)
	}
}

func splitNumberAndUnit(token string) (string, string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ""
	}
	for i, r := range token {
		if !(unicode.IsDigit(r) || r == '.' || r == '-' || r == '+') {
			return strings.TrimSpace(token[:i]), strings.TrimSpace(token[i:])
		}
	}
	return token, ""
}

func blockLinesToParagraphs(block []rawBlockLine) []string {
	if len(block) == 0 {
		return nil
	}
	var paragraphs []string
	var current []string
	for _, line := range block {
		if strings.TrimSpace(line.Text) == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, strings.Join(current, "\n"))
				current = nil
			}
			continue
		}
		current = append(current, strings.TrimRight(line.Text, " \t"))
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, strings.Join(current, "\n"))
	}
	if len(paragraphs) == 0 {
		return []string{""}
	}
	return paragraphs
}

// BatchLogParseError annotates parse failures with the originating line number.
type BatchLogParseError struct {
	Line int
	Err  error
}

func (e BatchLogParseError) Error() string {
	if e.Line <= 0 {
		return e.Err.Error()
	}
	return fmt.Sprintf("batch log line %d: %v", e.Line, e.Err)
}

// Unwrap exposes the underlying parse error.
func (e BatchLogParseError) Unwrap() error {
	return e.Err
}
