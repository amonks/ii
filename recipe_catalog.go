package creamery

import (
	"errors"
	"os"
	"strings"
	"time"
)

// RecipeCatalogEntry references a recorded formulation along with derived data.
type RecipeCatalogEntry struct {
	Label    string
	Recipe   *Recipe
	Snapshot *BatchSnapshot
	Issues   []string
	Raw      BatchLogEntry
}

// RecipeCatalog aggregates every known recipe source.
type RecipeCatalog struct {
	SourcePath string
	Entries    []RecipeCatalogEntry

	rawEntries []BatchLogEntry
}

// BatchLogEntries exposes the underlying batch-log representation.
func (rc RecipeCatalog) BatchLogEntries() []BatchLogEntry {
	out := make([]BatchLogEntry, len(rc.rawEntries))
	copy(out, rc.rawEntries)
	return out
}

// LoadRecipeCatalogFromBatchLog ingests the canonical batch log file.
func LoadRecipeCatalogFromBatchLog(path string, catalog IngredientCatalog) (RecipeCatalog, error) {
	return LoadRecipeCatalogFromFiles([]string{path}, catalog)
}

// LoadRecipeCatalogFromFiles ingests multiple batch-log style files (e.g. production log + curated recipes).
func LoadRecipeCatalogFromFiles(paths []string, catalog IngredientCatalog) (RecipeCatalog, error) {
	allEntries := make([]BatchLogEntry, 0)
	usedPaths := make([]string, 0, len(paths))

	for _, rawPath := range paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}
		entries, err := loadBatchEntries(path)
		if err != nil {
			return RecipeCatalog{}, err
		}
		if len(entries) == 0 {
			continue
		}
		for _, entry := range entries {
			allEntries = append(allEntries, entry)
		}
		usedPaths = append(usedPaths, path)
	}

	for i := range allEntries {
		allEntries[i].Sequence = i + 1
	}

	return buildRecipeCatalog(allEntries, usedPaths, catalog)
}

func loadBatchEntries(path string) ([]BatchLogEntry, error) {
	entries, err := LoadBatchesFromDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return entries, nil
}

func buildRecipeCatalog(entries []BatchLogEntry, sources []string, catalog IngredientCatalog) (RecipeCatalog, error) {
	catalogResult := RecipeCatalog{
		SourcePath: strings.Join(sources, ", "),
		Entries:    make([]RecipeCatalogEntry, 0, len(entries)),
		rawEntries: make([]BatchLogEntry, len(entries)),
	}
	copy(catalogResult.rawEntries, entries)

	for _, entry := range entries {
		label := strings.TrimSpace(entry.Recipe)
		if label == "" {
			label = entry.Label()
		}
		item := RecipeCatalogEntry{
			Label: label,
			Raw:   entry,
		}
		components, err := entry.Components(catalog)
		if err != nil {
			item.Issues = append(item.Issues, err.Error())
			catalogResult.Entries = append(catalogResult.Entries, item)
			continue
		}
		snapshot, snapErr := BuildProperties(components, MixOptions{})
		if snapErr != nil {
			item.Issues = append(item.Issues, snapErr.Error())
		} else {
			itemSnapshot := snapshot
			item.Snapshot = &itemSnapshot
		}
		recipe, recipeErr := NewRecipe(components, 0)
		if recipeErr != nil {
			item.Issues = append(item.Issues, recipeErr.Error())
		} else {
			item.Recipe = recipe
		}
		catalogResult.Entries = append(catalogResult.Entries, item)
	}

	return catalogResult, nil
}

// RecipeCatalogAnalysis summarizes chemistry and usage across the catalog.
type RecipeCatalogAnalysis struct {
	GeneratedAt time.Time
	Analytics   BatchLogAnalytics
}

// AnalyzeRecipeCatalog reuses the batch-log analytics to score recipes.
func AnalyzeRecipeCatalog(rc RecipeCatalog, catalog IngredientCatalog) RecipeCatalogAnalysis {
	analytics := AnalyzeBatchLog(rc.BatchLogEntries(), catalog)
	return RecipeCatalogAnalysis{
		GeneratedAt: time.Now(),
		Analytics:   analytics,
	}
}
