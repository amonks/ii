package creamery

import (
	"fmt"
	"time"
)

// LabelCatalogEntry captures metadata for a reverse-engineering scenario.
type LabelCatalogEntry struct {
	ID     string
	Name   string
	Build  func() (*LabelScenarioResult, error)
	Hidden bool
}

// DefaultLabelCatalog exposes the built-in label scenarios in a stable order.
func DefaultLabelCatalog() []LabelCatalogEntry {
	return []LabelCatalogEntry{
		{ID: "ben", Name: "Ben & Jerry's Vanilla", Build: SolveBenAndJerryVanilla},
		{ID: "jenis", Name: "Jeni's Sweet Cream", Build: SolveJenisSweetCream},
		{ID: "haagen", Name: "Haagen-Dazs Vanilla", Build: SolveHaagenDazsVanilla},
		{ID: "brighams", Name: "Brigham's Vanilla", Build: SolveBrighamsVanilla},
		{ID: "breyers", Name: "Breyers Vanilla", Build: SolveBreyersVanilla},
		{ID: "talenti", Name: "Talenti Vanilla Bean", Build: SolveTalentiVanilla},
	}
}

// LabelCatalogAnalysis aggregates diagnostics for every label scenario.
type LabelCatalogAnalysis struct {
	GeneratedAt time.Time
	Entries     []LabelAnalysisEntry
}

// LabelAnalysisEntry reports status for a single label scenario solve.
type LabelAnalysisEntry struct {
	Entry    LabelCatalogEntry
	Result   *LabelScenarioResult
	Err      error
	Duration time.Duration
}

// AnalyzeLabelCatalog executes all catalog builders and records the outcome.
func AnalyzeLabelCatalog(entries []LabelCatalogEntry) LabelCatalogAnalysis {
	report := LabelCatalogAnalysis{
		GeneratedAt: time.Now(),
		Entries:     make([]LabelAnalysisEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		start := time.Now()
		res, err := entry.Build()
		item := LabelAnalysisEntry{
			Entry:    entry,
			Result:   res,
			Err:      err,
			Duration: time.Since(start),
		}
		if item.Err != nil {
			item.Err = fmt.Errorf("%s: %w", entry.Name, item.Err)
		}
		report.Entries = append(report.Entries, item)
	}
	return report
}
