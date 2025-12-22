package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/creamery"
	batchlogtemplates "github.com/amonks/creamery/internal/templates/batchlog"
)

var (
	logPath   = flag.String("log", "batchlog", "Path to the batch log file")
	serveAddr = flag.String("serve", "", "HTTP address to serve the HTML dashboard (example :8080)")
)

const dashboardTemplate = "index.html.tmpl"

func main() {
	flag.Parse()
	log.SetFlags(0)

	entries, err := loadEntries(*logPath)
	if err != nil {
		log.Fatal(err)
	}
	catalog := creamery.DefaultIngredientCatalog()
	analytics := creamery.AnalyzeBatchLog(entries, catalog)

	printSummary(os.Stdout, *logPath, analytics)
	if analytics.Summary.TotalBatches > 0 {
		printEntryDetails(os.Stdout, analytics)
	}

	if *serveAddr == "" {
		return
	}
	tmpl := template.Must(loadDashboardTemplate())
	server := &dashboardServer{
		logPath: *logPath,
		tmpl:    tmpl,
		catalog: catalog,
	}
	addr := *serveAddr
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	log.Printf("Serving batch log dashboard for %s at %s\n", *logPath, serveURL(addr))
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}

func loadEntries(path string) ([]creamery.BatchLogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()
	entries, err := creamery.ParseBatchLog(f)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func printSummary(w io.Writer, path string, analytics creamery.BatchLogAnalytics) {
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

func printEntryDetails(w io.Writer, analytics creamery.BatchLogAnalytics) {
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
		if view.Snapshot != nil {
			s := view.Snapshot
			fmt.Fprintf(w, "    mass %.2f kg | water %.2f%% | solids %.2f%% | fat %.2f%% | protein %.2f%%\n",
				s.TotalMassKg, s.WaterPct*100, s.SolidsPct*100, s.FatPct*100, s.ProteinPct*100)
			fmt.Fprintf(w, "    sugars %.2f%% (added %.2f%%) | lactose %.2f%% | cost %s\n",
				s.TotalSugarsPct*100, s.AddedSugarsPct*100, s.LactosePct*100, formatCurrencyPerKg(s.CostPerKg))
			fmt.Fprintf(w, "    freezing %.2f°C | ice %.1f%% | viscosity %.4f Pa·s | overrun %.1f%% | hardness %.1f | meltdown %.1f\n",
				s.FreezingPointC, s.IceFractionAtServe*100, s.ViscosityAtServe, s.OverrunEstimate*100, s.HardnessIndex, s.MeltdownIndex)
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

type dashboardServer struct {
	logPath string
	tmpl    *template.Template
	catalog creamery.IngredientCatalog
}

func (s *dashboardServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entries, err := loadEntries(s.logPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	analytics := creamery.AnalyzeBatchLog(entries, s.catalog)
	data := pageData{
		GeneratedAt: time.Now(),
		SourcePath:  filepath.Clean(s.logPath),
		Analytics:   analytics,
	}
	if err := s.tmpl.ExecuteTemplate(w, dashboardTemplate, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type pageData struct {
	GeneratedAt time.Time
	SourcePath  string
	Analytics   creamery.BatchLogAnalytics
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatDate":          formatDate,
		"formatDateTime":      formatDateTime,
		"formatPercent":       formatPercent,
		"formatCurrencyPerKg": formatCurrencyPerKg,
		"join": func(values []string, sep string) string {
			return strings.Join(values, sep)
		},
	}
}

func loadDashboardTemplate() (*template.Template, error) {
	return template.New(dashboardTemplate).
		Funcs(templateFuncs()).
		ParseFS(batchlogtemplates.Files, dashboardTemplate)
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

func formatPercent(v float64) string {
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

func serveURL(addr string) string {
	if addr == "" {
		return "http://localhost"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" {
		host = "localhost"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if port == "" {
		return "http://" + host
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}
