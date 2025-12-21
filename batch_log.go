package creamery

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// BatchLogEntry captures a single production record sourced from the batch log.
type BatchLogEntry struct {
	Sequence     int
	Date         time.Time
	Recipe       string
	Ingredients  []BatchLogIngredient
	ProcessNotes []string
	TastingNotes []string
}

// BatchLogIngredient couples an ingredient catalog key with a realized mass.
type BatchLogIngredient struct {
	Key     IngredientKey
	MassKg  float64
	RawMass string
	Line    int
}

// Components converts the entry's ingredient weights into recipe components using the provided catalog.
func (e BatchLogEntry) Components(catalog IngredientCatalog) ([]RecipeComponent, error) {
	if len(e.Ingredients) == 0 {
		return nil, fmt.Errorf("batch %s has no ingredient weights", e.Label())
	}

	components := make([]RecipeComponent, 0, len(e.Ingredients))
	for _, entry := range e.Ingredients {
		if entry.MassKg <= 0 {
			continue
		}
		lot, ok := catalog.InstanceByKey(entry.Key.String())
		if !ok || lot.Definition == nil {
			return nil, fmt.Errorf("batch %s references unknown ingredient %q", e.Label(), entry.Key)
		}
		components = append(components, RecipeComponent{
			Ingredient: lot,
			MassKg:     entry.MassKg,
		})
	}
	if len(components) == 0 {
		return nil, fmt.Errorf("batch %s has no positive ingredient weights", e.Label())
	}
	return components, nil
}

// Batch normalizes the entry into a Batch for downstream chemistry analysis.
func (e BatchLogEntry) Batch(catalog IngredientCatalog) (Batch, error) {
	components, err := e.Components(catalog)
	if err != nil {
		return Batch{}, err
	}
	portions, total := PortionsFromMasses(componentsToPortionMasses(components))
	if total <= 0 {
		return Batch{}, errors.New("batch has zero total mass")
	}
	return BatchFromPortions(portions, total), nil
}

// Snapshot aggregates the batch entry into the stock BatchSnapshot structure and applies process calculations.
func (e BatchLogEntry) Snapshot(catalog IngredientCatalog) (BatchSnapshot, error) {
	components, err := e.Components(catalog)
	if err != nil {
		return BatchSnapshot{}, err
	}
	return BuildProperties(components, MixOptions{})
}

// Label returns a human-readable identifier for the entry.
func (e BatchLogEntry) Label() string {
	if e.Sequence > 0 {
		label := fmt.Sprintf("#%d", e.Sequence)
		if !e.Date.IsZero() {
			return fmt.Sprintf("%s %s", label, e.Date.Format("2006-01-02"))
		}
		return label
	}
	return "#0"
}

func componentsToPortionMasses(components []RecipeComponent) []PortionMass {
	out := make([]PortionMass, 0, len(components))
	for _, comp := range components {
		out = append(out, PortionMass{
			Lot:    comp.Ingredient,
			MassKg: comp.MassKg,
		})
	}
	return out
}

// BatchLogAnalytics captures per-entry snapshots and aggregate rollups for reporting.
type BatchLogAnalytics struct {
	Entries []BatchLogEntryView
	Summary BatchLogSummary
}

// BatchLogEntryView decorates a raw entry with computed metrics and any analysis issues.
type BatchLogEntryView struct {
	Entry      BatchLogEntry
	Snapshot   *BatchSnapshot
	Components []RecipeComponent
	Issues     []string
}

// BatchLogSummary aggregates totals across the parsed log.
type BatchLogSummary struct {
	TotalBatches       int
	EarliestDate       time.Time
	LatestDate         time.Time
	ValidSnapshots     int
	InvalidEntries     int
	IngredientTotals   []BatchLogIngredientUsage
	EntriesWithIssues  int
	EntriesMissingDate int
}

// BatchLogIngredientUsage summarizes how often an ingredient appears across the log.
type BatchLogIngredientUsage struct {
	Key         IngredientKey
	Name        string
	TotalMassKg float64
	BatchCount  int
}

// AnalyzeBatchLog builds per-entry snapshots and summary analytics in a single pass.
func AnalyzeBatchLog(entries []BatchLogEntry, catalog IngredientCatalog) BatchLogAnalytics {
	result := BatchLogAnalytics{
		Entries: make([]BatchLogEntryView, 0, len(entries)),
	}

	totals := make(map[IngredientKey]*BatchLogIngredientUsage)
	var snapshotCount int
	var issuesCount int
	var missingDateCount int
	var earliest, latest time.Time

	for _, entry := range entries {
		view := BatchLogEntryView{
			Entry: entry,
		}
		comps, err := entry.Components(catalog)
		if err != nil {
			view.Issues = append(view.Issues, err.Error())
			issuesCount++
			result.Entries = append(result.Entries, view)
			if entry.Date.IsZero() {
				missingDateCount++
			} else {
				updateDateRange(&earliest, &latest, entry.Date)
			}
			continue
		}
		view.Components = comps

		snapshot, err := BuildProperties(comps, MixOptions{})
		if err != nil {
			view.Issues = append(view.Issues, err.Error())
			issuesCount++
		} else {
			view.Snapshot = &snapshot
			snapshotCount++
		}

		if entry.Date.IsZero() {
			missingDateCount++
		} else {
			updateDateRange(&earliest, &latest, entry.Date)
		}

		perEntry := make(map[IngredientKey]bool)
		for _, comp := range comps {
			key := IngredientKey("")
			if comp.Ingredient.Definition != nil && comp.Ingredient.Definition.Key != "" {
				key = comp.Ingredient.Definition.Key
			} else if comp.Ingredient.Definition != nil {
				key = IngredientKey(comp.Ingredient.Definition.ID)
			}
			if key == "" {
				key = IngredientKey(NewIngredientKey(comp.Ingredient.DisplayName()))
			}
			usage := totals[key]
			if usage == nil {
				usage = &BatchLogIngredientUsage{
					Key:  key,
					Name: comp.Ingredient.DisplayName(),
				}
				totals[key] = usage
			}
			usage.TotalMassKg += comp.MassKg
			if !perEntry[key] {
				usage.BatchCount++
				perEntry[key] = true
			}
		}

		result.Entries = append(result.Entries, view)
	}

	summary := BatchLogSummary{
		TotalBatches:       len(entries),
		EarliestDate:       earliest,
		LatestDate:         latest,
		ValidSnapshots:     snapshotCount,
		InvalidEntries:     len(entries) - snapshotCount,
		EntriesWithIssues:  issuesCount,
		EntriesMissingDate: missingDateCount,
	}
	usage := make([]BatchLogIngredientUsage, 0, len(totals))
	for _, stat := range totals {
		usage = append(usage, *stat)
	}
	sort.Slice(usage, func(i, j int) bool {
		if usage[i].TotalMassKg == usage[j].TotalMassKg {
			return usage[i].Name < usage[j].Name
		}
		return usage[i].TotalMassKg > usage[j].TotalMassKg
	})
	summary.IngredientTotals = usage

	result.Summary = summary
	return result
}

func updateDateRange(earliest, latest *time.Time, candidate time.Time) {
	if candidate.IsZero() {
		return
	}
	if earliest.IsZero() || candidate.Before(*earliest) {
		*earliest = candidate
	}
	if latest.IsZero() || candidate.After(*latest) {
		*latest = candidate
	}
}
