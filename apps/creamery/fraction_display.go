package creamery

import (
	"fmt"
	"math"
	"strings"
)

type fractionDisplayGroup struct {
	Keys         []string
	DefaultLabel string
	LabelFunc    func(total float64, values map[string]float64) string
}

var fractionDisplayGroups = []fractionDisplayGroup{
	{
		Keys:         []string{"cream_fat", "cream_serum"},
		DefaultLabel: "cream",
		LabelFunc:    creamDisplayLabel,
	},
}

// CombineFractionDisplayAliases collapses related fraction entries into a single
// display row while preserving meaningful context (e.g. cream fat + cream serum
// -> “36% cream”). The input map is left unchanged; a new map is returned.
func CombineFractionDisplayAliases(fractions map[string]float64) map[string]float64 {
	if len(fractions) == 0 {
		return fractions
	}
	display := make(map[string]float64, len(fractions))
	for name, value := range fractions {
		if math.Abs(value) < 1e-9 {
			continue
		}
		display[name] = value
	}
	for _, group := range fractionDisplayGroups {
		values := make(map[string]float64, len(group.Keys))
		total := 0.0
		active := 0
		for _, key := range group.Keys {
			if value, ok := display[key]; ok && value > 0 {
				values[key] = value
				total += value
				active++
			}
		}
		if active < len(group.Keys) || total <= 0 {
			continue
		}
		for key := range values {
			delete(display, key)
		}
		label := group.DefaultLabel
		if group.LabelFunc != nil {
			if custom := strings.TrimSpace(group.LabelFunc(total, values)); custom != "" {
				label = custom
			}
		}
		display[label] += total
	}
	return display
}

func creamDisplayLabel(total float64, values map[string]float64) string {
	if total <= 0 {
		return "cream"
	}
	fat := values["cream_fat"]
	pct := 100 * fat / total
	return formatCreamPercentage(pct)
}

func formatCreamPercentage(percent float64) string {
	percent = math.Max(0, math.Min(100, percent))
	rounded := math.Round(percent*10) / 10
	if math.Abs(rounded-math.Round(rounded)) < 1e-4 {
		return fmt.Sprintf("%.0f%% cream", math.Round(rounded))
	}
	return fmt.Sprintf("%.1f%% cream", rounded)
}
