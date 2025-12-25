package creamery

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

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

type massMeasurement struct {
	ValueKg     float64
	PrecisionKg float64
	Unit        string
}

func parseMassValue(input string) (massMeasurement, error) {
	clean := strings.TrimSpace(stripInlineComment(input))
	if clean == "" {
		return massMeasurement{}, errors.New("missing mass value")
	}

	numberToken, unitToken, err := extractNumberAndUnit(clean)
	if err != nil {
		return massMeasurement{}, err
	}
	value, err := strconv.ParseFloat(numberToken, 64)
	if err != nil {
		return massMeasurement{}, fmt.Errorf("invalid mass %q", clean)
	}
	unit, factor, err := normalizeUnit(unitToken)
	if err != nil {
		return massMeasurement{}, err
	}
	resolution := resolutionFromNumber(numberToken)
	return massMeasurement{
		ValueKg:     value * factor,
		PrecisionKg: resolution * factor,
		Unit:        unit,
	}, nil
}

func stripInlineComment(input string) string {
	if idx := strings.Index(input, "#"); idx >= 0 {
		return input[:idx]
	}
	return input
}

func extractNumberAndUnit(value string) (string, string, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "", "", errors.New("missing mass value")
	}
	if len(fields) == 1 {
		num, unit := splitNumberAndUnit(fields[0])
		if num == "" {
			return "", "", errors.New("missing numeric mass value")
		}
		return num, unit, nil
	}
	return fields[0], strings.Join(fields[1:], ""), nil
}

func normalizeUnit(unit string) (string, float64, error) {
	unit = strings.Trim(strings.ToLower(unit), ". ")
	switch unit {
	case "", "kg", "kilogram", "kilograms":
		return "kg", 1, nil
	case "g", "gram", "grams":
		return "g", 1.0 / 1000.0, nil
	case "mg", "milligram", "milligrams":
		return "mg", 1.0 / 1_000_000.0, nil
	case "lb", "lbs", "pound", "pounds":
		return "lb", 0.45359237, nil
	case "oz", "ounce", "ounces":
		return "oz", 0.028349523125, nil
	default:
		return "", 0, fmt.Errorf("unknown mass unit %q", unit)
	}
}

func resolutionFromNumber(token string) float64 {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0
	}
	trimmed := token
	exponent := 0
	if idx := strings.IndexAny(trimmed, "eE"); idx >= 0 {
		expPart := strings.TrimSpace(trimmed[idx+1:])
		trimmed = trimmed[:idx]
		if expPart != "" {
			if expVal, err := strconv.Atoi(expPart); err == nil {
				exponent = expVal
			}
		}
	}
	decimals := 0
	if dot := strings.Index(trimmed, "."); dot >= 0 {
		decimals = len(trimmed) - dot - 1
	}
	resolution := math.Pow10(-decimals)
	if exponent != 0 {
		resolution *= math.Pow10(exponent)
	}
	return resolution
}

func splitNumberAndUnit(token string) (string, string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ""
	}
	for i, r := range token {
		if !(unicode.IsDigit(r) || r == '.' || r == '-' || r == '+' || r == 'e' || r == 'E') {
			return strings.TrimSpace(token[:i]), strings.TrimSpace(token[i:])
		}
	}
	return token, ""
}

func formatPrecisionDisplay(precisionKg float64, unit string) string {
	if precisionKg <= 0 {
		return ""
	}
	var value float64
	switch unit {
	case "kg":
		value = precisionKg
	case "g":
		value = precisionKg * 1000
	case "mg":
		value = precisionKg * 1_000_000
	case "lb":
		value = precisionKg / 0.45359237
	case "oz":
		value = precisionKg / 0.028349523125
	default:
		value = precisionKg
		unit = "kg"
	}
	return fmt.Sprintf("%s %s", formatPrecisionValue(value), unit)
}

func formatPrecisionValue(v float64) string {
	if v == 0 {
		return "0"
	}
	switch {
	case v >= 1:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
	case v >= 0.01:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
	case v >= 0.0001:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
	default:
		return fmt.Sprintf("%.2e", v)
	}
}
