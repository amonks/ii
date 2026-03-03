package creamery

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"monks.co/pkg/serve"
)

const batchLogTemplateName = "batchlog_index.html.tmpl"

// LoadBatchLogEntries parses batch entries from a directory of .batch files.
func LoadBatchLogEntries(path string) ([]BatchLogEntry, error) {
	return LoadBatchesFromDir(path)
}

// LoadBatchLogTemplate prepares the shared HTML dashboard template.
func LoadBatchLogTemplate() (*template.Template, error) {
	return template.New(batchLogTemplateName).
		Funcs(batchLogTemplateFuncs()).
		ParseFS(templateFiles, "base_styles.tmpl", batchLogTemplateName)
}

// NewBatchLogDashboard wires the HTTP handler used by CLI and server modes.
func NewBatchLogDashboard(logPath string, catalog IngredientCatalog) (*BatchLogDashboard, error) {
	tmpl, err := LoadBatchLogTemplate()
	if err != nil {
		return nil, err
	}
	return &BatchLogDashboard{
		logPath: logPath,
		catalog: catalog,
		tmpl:    tmpl,
	}, nil
}

// BatchLogDashboard renders the batch-log analytics as HTML.
type BatchLogDashboard struct {
	logPath string
	catalog IngredientCatalog
	tmpl    *template.Template
}

// ServeHTTP implements http.Handler.
func (d *BatchLogDashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entries, err := LoadBatchLogEntries(d.logPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	analytics := AnalyzeBatchLog(entries, d.catalog)
	data := struct {
		GeneratedAt time.Time
		SourcePath  string
		Analytics   BatchLogAnalytics
		BasePath    string
	}{
		GeneratedAt: time.Now(),
		SourcePath:  filepath.Clean(d.logPath),
		Analytics:   analytics,
		BasePath:    serve.BasePath(r),
	}
	if err := d.tmpl.ExecuteTemplate(w, batchLogTemplateName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// PrintBatchLogSummary mirrors the previous CLI summary printer.
func PrintBatchLogSummary(w io.Writer, path string, analytics BatchLogAnalytics) {
	fmt.Fprintf(w, "Batch log: %s\n", path)
	if analytics.Summary.TotalBatches == 0 {
		fmt.Fprintln(w, "  (no batches logged yet)")
		return
	}
	fmt.Fprintf(w, "  Total batches : %d (valid %d, invalid %d)\n",
		analytics.Summary.TotalBatches,
		analytics.Summary.ValidSnapshots,
		analytics.Summary.InvalidEntries)
	if !analytics.Summary.EarliestDate.IsZero() {
		fmt.Fprintf(w, "  Date range    : %s — %s\n",
			analytics.Summary.EarliestDate.Format("2006-01-02"),
			analytics.Summary.LatestDate.Format("2006-01-02"))
	}
	fmt.Fprintf(w, "  Issues        : %d entries with warnings, %d missing dates\n",
		analytics.Summary.EntriesWithIssues,
		analytics.Summary.EntriesMissingDate)

	fmt.Fprintln(w, "\nTop ingredients:")
	top := analytics.Summary.IngredientTotals
	if len(top) == 0 {
		fmt.Fprintln(w, "  (no ingredient data)")
		return
	}
	limit := 5
	if len(top) < limit {
		limit = len(top)
	}
	for i := 0; i < limit; i++ {
		entry := top[i]
		fmt.Fprintf(w, "  - %-20s %6.2f kg across %d batches\n", entry.Name, entry.TotalMassKg, entry.BatchCount)
	}
}

// PrintBatchLogEntries dumps per-entry diagnostics similar to the old CLI.
func PrintBatchLogEntries(w io.Writer, analytics BatchLogAnalytics) {
	fmt.Fprintln(w, "\nEntries and chemistry:")
	if len(analytics.Entries) == 0 {
		fmt.Fprintln(w, "  (no entries available)")
		return
	}
	for _, view := range analytics.Entries {
		entry := view.Entry
		label := entry.Label()
		fmt.Fprintf(w, "- %s", label)
		if entry.Recipe != "" {
			fmt.Fprintf(w, " • %s", entry.Recipe)
		}
		fmt.Fprintln(w)
		if len(entry.Ingredients) > 0 {
			fmt.Fprintln(w, "    ingredients:")
			for _, ing := range entry.Ingredients {
				fmt.Fprintf(w, "      • %s: %s", ing.Key, ing.RawMass)
				if ing.PrecisionDisplay != "" {
					fmt.Fprintf(w, " (±%s)", ing.PrecisionDisplay)
				}
				fmt.Fprintln(w)
			}
		}
		if view.Snapshot != nil && view.Process != nil {
			s := view.Snapshot
			p := view.Process
			fmt.Fprintf(w, "    mass %.2f kg | water %.2f%% | solids %.2f%% | fat %.2f%% | protein %.2f%%\n",
				s.TotalMassKg, s.WaterPct()*100, s.SolidsPct()*100, s.FatPct()*100, s.ProteinPct()*100)
			fmt.Fprintf(w, "    sugars %.2f%% (added %.2f%%) | lactose %.2f%% | cost %s\n",
				s.TotalSugarsPct()*100, s.AddedSugarsPct()*100, s.LactosePct()*100, formatCurrencyPerKg(s.CostPerKg()))
			fmt.Fprintf(w, "    freezing %.2f°C | ice %.1f%% | viscosity %.4f Pa·s | overrun %.1f%% | hardness %.1f | meltdown %.1f\n",
				p.FreezingPointC, p.IceFractionAtServe*100, p.ViscosityAtServe, p.OverrunEstimate*100, p.HardnessIndex, p.MeltdownIndex)
		} else if len(view.Issues) > 0 {
			fmt.Fprintf(w, "    issues: %s\n", strings.Join(view.Issues, "; "))
		}
		if len(entry.ProcessNotes) > 0 {
			fmt.Fprintln(w, "    process:")
			for _, note := range entry.ProcessNotes {
				fmt.Fprintf(w, "      • %s\n", indentMultiline(note, "        "))
			}
		}
		if len(entry.TastingNotes) > 0 {
			fmt.Fprintln(w, "    tasting:")
			for _, note := range entry.TastingNotes {
				fmt.Fprintf(w, "      • %s\n", indentMultiline(note, "        "))
			}
		}
		if len(view.Issues) > 0 && view.Snapshot != nil {
			fmt.Fprintf(w, "    issues: %s\n", strings.Join(view.Issues, "; "))
		}
		fmt.Fprintln(w)
	}
}

func batchLogTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatDate":          formatDate,
		"formatDateTime":      formatDateTime,
		"formatPercent":       renderPercent,
		"formatCurrencyPerKg": formatCurrencyPerKg,
		"join": func(values []string, sep string) string {
			return strings.Join(values, sep)
		},
	}
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006")
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Mon Jan 2 15:04:05 MST 2006")
}

func renderPercent(v float64) string {
	if v <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.2f%%", v*100)
}

func formatCurrencyPerKg(v float64) string {
	if v <= 0 {
		return "—"
	}
	return fmt.Sprintf("$%.2f/kg", v)
}

func indentMultiline(text, indent string) string {
	if !strings.Contains(text, "\n") {
		return text
	}
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

