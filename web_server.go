package creamery

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// UnifiedServer renders labels, recipes, and batch-log analytics from one host.
type UnifiedServer struct {
	catalog        IngredientCatalog
	labelCatalog   []LabelCatalogEntry
	batchLogPath   string
	recipeLogPath  string
	batchDashboard *BatchLogDashboard

	homeTmpl    *template.Template
	labelsTmpl  *template.Template
	recipesTmpl *template.Template
}

// NewUnifiedServer wires the HTTP handlers needed by the consolidated CLI.
func NewUnifiedServer(batchLogPath, recipeLogPath string, catalog IngredientCatalog, labelCatalog []LabelCatalogEntry) (*UnifiedServer, error) {
	dashboard, err := NewBatchLogDashboard(batchLogPath, catalog)
	if err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"formatPercent":  renderPercent,
		"formatDuration": func(d time.Duration) string { return d.Truncate(time.Millisecond).String() },
		"formatDate":     formatDate,
		"formatDateTime": formatDateTime,
	}

	home := template.Must(template.New("home").Funcs(funcs).Parse(homePageTemplate))
	labels := template.Must(template.New("labels").Funcs(funcs).Parse(labelsPageTemplate))
	recipes := template.Must(template.New("recipes").Funcs(funcs).Parse(recipesPageTemplate))

	return &UnifiedServer{
		catalog:        catalog,
		labelCatalog:   labelCatalog,
		batchLogPath:   batchLogPath,
		recipeLogPath:  recipeLogPath,
		batchDashboard: dashboard,
		homeTmpl:       home,
		labelsTmpl:     labels,
		recipesTmpl:    recipes,
	}, nil
}

// ServeHTTP routes traffic across the analytics dashboards.
func (s *UnifiedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "", "/":
		s.renderHome(w, r)
	case "/labels":
		s.renderLabels(w, r)
	case "/recipes":
		s.renderRecipes(w, r)
	case "/batchlog":
		s.batchDashboard.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *UnifiedServer) renderHome(w http.ResponseWriter, r *http.Request) {
	data := homePageData{
		GeneratedAt: time.Now(),
		BatchLog:    s.batchLogPath,
		LabelCount:  len(s.labelCatalog),
	}
	_ = s.homeTmpl.Execute(w, data)
}

func (s *UnifiedServer) renderLabels(w http.ResponseWriter, r *http.Request) {
	report := AnalyzeLabelCatalog(s.labelCatalog)
	data := labelPageData{
		GeneratedAt: report.GeneratedAt,
		CatalogSize: len(report.Entries),
		Entries:     make([]labelCardData, 0, len(report.Entries)),
	}
	for _, entry := range report.Entries {
		card := labelCardData{
			ID:       entry.Entry.ID,
			Name:     entry.Entry.Name,
			Duration: entry.Duration,
		}
		def, _ := LabelScenarioByKey(entry.Entry.ID)
		facts := def.Facts
		if entry.Result != nil {
			facts = entry.Result.LabelFacts
		}
		nameLookup := ingredientNameLookup(def.IngredientSpecs)
		var fractionByID map[IngredientID]float64
		if entry.Result != nil && entry.Result.Recipe != nil {
			fractionByID = entry.Result.Recipe.batch().FractionsByID()
		}
		if len(def.DisplayNames) > 0 {
			card.IngredientOrder = def.DisplayNames
		} else if entry.Result != nil && len(entry.Result.LabelIngredients) > 0 {
			card.IngredientOrder = entry.Result.LabelIngredients
		}
		card.Presence = presenceNames(def.Presence, nameLookup, fractionByID)
		card.Groups = labelGroupsView(def.Groups, nameLookup, fractionByID)
		card.LabelFacts = labelFactRows(facts)
		card.FactComparisons = labelFactComparisons(facts, entry.Result)

		switch {
		case entry.Err != nil:
			card.Status = "Failed"
			card.StatusClass = "status-failed"
			card.Error = entry.Err.Error()
			card.IsError = true
			data.FailedCount++
		case entry.Result == nil:
			card.Status = "No Solution"
			card.StatusClass = "status-idle"
		default:
			card.Status = "Solved"
			card.StatusClass = "status-ok"
			card.Fractions = recipeFractions(entry.Result.Recipe)
			card.Sweetener = sweetenerRows(entry.Result.Metrics.Sweeteners)
			card.Metrics = processMetricRows(entry.Result.Metrics)
			card.SolverServing = measurement(entry.Result.ServingSizeGrams, "g", 1)
			card.PintMass = measurement(entry.Result.PintMassGrams, "g", 1)
			data.SolvedCount++
		}
		data.Entries = append(data.Entries, card)
	}
	if err := s.labelsTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *UnifiedServer) renderRecipes(w http.ResponseWriter, r *http.Request) {
	catalog, err := LoadRecipeCatalogFromFiles([]string{s.batchLogPath, s.recipeLogPath}, s.catalog)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	analysis := AnalyzeRecipeCatalog(catalog, s.catalog)
	data := recipePageData{
		GeneratedAt: analysis.GeneratedAt,
		Summary:     analysis.Analytics.Summary,
		Entries:     make([]recipeCardData, 0, len(catalog.Entries)),
	}
	for _, entry := range catalog.Entries {
		card := recipeCardData{
			Label:        entry.Label,
			Issues:       entry.Issues,
			HasIssues:    len(entry.Issues) > 0,
			ProcessNotes: entry.Raw.ProcessNotes,
			TastingNotes: entry.Raw.TastingNotes,
			Ingredients:  ingredientRows(entry.Raw.Ingredients, s.catalog),
			Fractions:    recipeFractions(entry.Recipe),
		}
		if !entry.Raw.Date.IsZero() {
			card.Date = formatDate(entry.Raw.Date)
		}
		if entry.Snapshot != nil {
			card.SnapshotStats = recipeSnapshotRows(entry.Snapshot)
			card.Sweetener = sweetenerRows(entry.Snapshot.Sweeteners)
			card.Meta = recipeMetaRows(entry.Snapshot)
		}
		data.Entries = append(data.Entries, card)
	}
	if err := s.recipesTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type homePageData struct {
	GeneratedAt time.Time
	BatchLog    string
	LabelCount  int
}

type labelPageData struct {
	GeneratedAt time.Time
	CatalogSize int
	SolvedCount int
	FailedCount int
	Entries     []labelCardData
}

type labelCardData struct {
	ID              string
	Name            string
	Duration        time.Duration
	Status          string
	StatusClass     string
	Error           string
	IsError         bool
	IngredientOrder []string
	Presence        []string
	Groups          []labelGroupView
	LabelFacts      []statRow
	FactComparisons []factComparison
	Fractions       []fractionView
	Sweetener       []statRow
	Metrics         []statRow
	SolverServing   string
	PintMass        string
}

type recipePageData struct {
	GeneratedAt time.Time
	Summary     BatchLogSummary
	Entries     []recipeCardData
}

type recipeCardData struct {
	Label         string
	Date          string
	Issues        []string
	HasIssues     bool
	ProcessNotes  []string
	TastingNotes  []string
	Ingredients   []ingredientRow
	Fractions     []fractionView
	SnapshotStats []statRow
	Sweetener     []statRow
	Meta          []statRow
}

type statRow struct {
	Label string
	Value string
}

type factComparison struct {
	Name  string
	Label string
	Pred  string
	Delta string
}

type fractionView struct {
	Name  string
	Value float64
}

type labelGroupView struct {
	Name         string
	Members      []string
	Notes        []string
	EnforceOrder bool
}

type ingredientRow struct {
	Name    string
	Mass    string
	Percent string
}

func ingredientNameLookup(specs []IngredientDefinition) map[IngredientID]string {
	names := make(map[IngredientID]string, len(specs))
	for _, spec := range specs {
		name := spec.Name
		if name == "" && spec.Profile.Name != "" {
			name = spec.Profile.Name
		}
		if name == "" && spec.ID != "" {
			name = spec.ID.String()
		}
		names[spec.ID] = name
	}
	return names
}

func namesFromIDs(ids []IngredientID, lookup map[IngredientID]string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := lookup[id]; ok && name != "" {
			out = append(out, name)
		} else {
			out = append(out, id.String())
		}
	}
	return out
}

func labelGroupsView(groups []LabelGroup, names map[IngredientID]string, fractions map[IngredientID]float64) []labelGroupView {
	result := make([]labelGroupView, 0, len(groups))
	for _, group := range groups {
		if len(group.Keys) == 0 {
			continue
		}
		view := labelGroupView{
			Name:         group.Name,
			Members:      groupMemberNames(group.Keys, names, fractions),
			EnforceOrder: group.EnforceInternalOrder,
		}
		if len(group.FractionBounds) > 0 {
			notes := make([]string, 0, len(group.FractionBounds))
			for id, bounds := range group.FractionBounds {
				label := names[id]
				if label == "" {
					label = id.String()
				}
				notes = append(notes, describeBounds(label, bounds))
			}
			sort.Strings(notes)
			view.Notes = notes
		}
		result = append(result, view)
	}
	return result
}

func describeBounds(name string, bounds Interval) string {
	lo := bounds.Lo
	hi := bounds.Hi
	switch {
	case lo > 0 && !math.IsInf(hi, 1):
		return fmt.Sprintf("%s between %s and %s of group", name, percentString(lo, 1), percentString(hi, 1))
	case lo > 0:
		return fmt.Sprintf("%s at least %s of group", name, percentString(lo, 1))
	case !math.IsInf(hi, 1):
		return fmt.Sprintf("%s at most %s of group", name, percentString(hi, 1))
	default:
		return fmt.Sprintf("%s unconstrained", name)
	}
}

func labelFactRows(facts NutritionFacts) []statRow {
	rows := make([]statRow, 0, 8)
	if facts.ServingSizeGrams > 0 {
		rows = append(rows, statRow{"Serving size", measurement(facts.ServingSizeGrams, "g", 1)})
	}
	if facts.Calories > 0 {
		rows = append(rows, statRow{"Calories", measurement(facts.Calories, "kcal", 0)})
	}
	if facts.TotalFatGrams > 0 {
		rows = append(rows, statRow{"Total fat", measurement(facts.TotalFatGrams, "g", 1)})
	}
	if facts.SaturatedFatGrams > 0 {
		rows = append(rows, statRow{"Saturated fat", measurement(facts.SaturatedFatGrams, "g", 1)})
	}
	if facts.TotalCarbGrams > 0 {
		rows = append(rows, statRow{"Total carbs", measurement(facts.TotalCarbGrams, "g", 1)})
	}
	if facts.TotalSugarsGrams > 0 {
		rows = append(rows, statRow{"Total sugars", measurement(facts.TotalSugarsGrams, "g", 1)})
	}
	if facts.AddedSugarsGrams > 0 {
		rows = append(rows, statRow{"Added sugars", measurement(facts.AddedSugarsGrams, "g", 1)})
	}
	if facts.ProteinGrams > 0 {
		rows = append(rows, statRow{"Protein", measurement(facts.ProteinGrams, "g", 1)})
	}
	if facts.SodiumMg > 0 {
		rows = append(rows, statRow{"Sodium", measurement(facts.SodiumMg, "mg", 0)})
	}
	if facts.CholesterolMg > 0 {
		rows = append(rows, statRow{"Cholesterol", measurement(facts.CholesterolMg, "mg", 0)})
	}
	return rows
}

func presenceNames(ids []IngredientID, names map[IngredientID]string, fractions map[IngredientID]float64) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	insertedCream := false
	for _, id := range ids {
		if isCreamComponent(id) {
			if insertedCream {
				continue
			}
			result = append(result, creamAliasLabel(fractions))
			insertedCream = true
			continue
		}
		result = append(result, ingredientDisplayNameForID(id, names))
	}
	return result
}

func groupMemberNames(ids []IngredientID, names map[IngredientID]string, fractions map[IngredientID]float64) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	insertedCream := false
	for _, id := range ids {
		if isCreamComponent(id) {
			if insertedCream {
				continue
			}
			result = append(result, creamAliasLabel(fractions))
			insertedCream = true
			continue
		}
		result = append(result, ingredientDisplayNameForID(id, names))
	}
	return result
}

func ingredientDisplayNameForID(id IngredientID, names map[IngredientID]string) string {
	if name, ok := names[id]; ok && name != "" {
		return name
	}
	return id.String()
}

func creamAliasLabel(fractions map[IngredientID]float64) string {
	if len(fractions) == 0 {
		return "cream"
	}
	fat := fractions[creamFatIngredientID]
	serum := fractions[creamSerumIngredientID]
	total := fat + serum
	if total <= 0 {
		return "cream"
	}
	return formatCreamPercentage(100 * fat / total)
}

func isCreamComponent(id IngredientID) bool {
	return id == creamFatIngredientID || id == creamSerumIngredientID
}

var (
	creamFatIngredientID   = NewIngredientID("cream_fat")
	creamSerumIngredientID = NewIngredientID("cream_serum")
)

func labelFactComparisons(actual NutritionFacts, result *LabelScenarioResult) []factComparison {
	if result == nil {
		return nil
	}
	pred := result.PredictedFacts
	rows := []struct {
		name     string
		unit     string
		decimals int
		label    float64
		guess    float64
	}{
		{"Calories", "kcal", 0, actual.Calories, pred.Calories},
		{"Total fat", "g", 2, actual.TotalFatGrams, pred.TotalFatGrams},
		{"Saturated fat", "g", 2, actual.SaturatedFatGrams, pred.SaturatedFatGrams},
		{"Total carbs", "g", 2, actual.TotalCarbGrams, pred.TotalCarbGrams},
		{"Total sugars", "g", 2, actual.TotalSugarsGrams, pred.TotalSugarsGrams},
		{"Added sugars", "g", 2, actual.AddedSugarsGrams, pred.AddedSugarsGrams},
		{"Protein", "g", 2, actual.ProteinGrams, pred.ProteinGrams},
	}
	out := make([]factComparison, 0, len(rows))
	for _, row := range rows {
		if row.label == 0 && row.guess == 0 {
			continue
		}
		out = append(out, factComparison{
			Name:  row.name,
			Label: measurement(row.label, row.unit, row.decimals),
			Pred:  measurement(row.guess, row.unit, row.decimals),
			Delta: deltaString(row.label, row.guess, row.unit, row.decimals),
		})
	}
	return out
}

func measurement(value float64, unit string, decimals int) string {
	format := fmt.Sprintf("%%.%df %%s", decimals)
	return fmt.Sprintf(format, value, unit)
}

func deltaString(label, pred float64, unit string, decimals int) string {
	diff := pred - label
	epsilon := math.Pow(10, float64(-(decimals + 1)))
	if math.Abs(diff) < epsilon {
		return fmt.Sprintf("0 %s", unit)
	}
	format := fmt.Sprintf("%%+.%df %%s", decimals)
	return fmt.Sprintf(format, diff, unit)
}

func percentString(value float64, decimals int) string {
	format := fmt.Sprintf("%%.%df%%%%", decimals)
	return fmt.Sprintf(format, value*100)
}

func recipeFractions(recipe *Recipe) []fractionView {
	if recipe == nil {
		return nil
	}
	fractions := CombineFractionDisplayAliases(recipe.Fractions())
	out := make([]fractionView, 0, len(fractions))
	for name, value := range fractions {
		if value < 1e-4 {
			continue
		}
		out = append(out, fractionView{Name: name, Value: value})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Value > out[j].Value
	})
	return out
}

func sweetenerRows(analysis SweetenerAnalysis) []statRow {
	if analysis.TotalPOD == 0 && analysis.TotalPAC == 0 && analysis.AddedSugarPOD == 0 && analysis.LactosePOD == 0 {
		return nil
	}
	return []statRow{
		{"Total POD", formatDecimal(analysis.TotalPOD, 1)},
		{"Total PAC", formatDecimal(analysis.TotalPAC, 1)},
		{"Added sugar POD", formatDecimal(analysis.AddedSugarPOD, 1)},
		{"Lactose POD", formatDecimal(analysis.LactosePOD, 1)},
		{"Added sugar PAC", formatDecimal(analysis.AddedSugarPAC, 1)},
		{"Lactose PAC", formatDecimal(analysis.LactosePAC, 1)},
		{"Sucrose equivalent", percentString(analysis.EquivalentSucrose(), 2)},
		{"Softness", analysis.RelativeSoftness()},
	}
}

func processMetricRows(snapshot BatchSnapshot) []statRow {
	if snapshot.TotalMassKg == 0 {
		return nil
	}
	return []statRow{
		{"Freezing point", fmt.Sprintf("%s °C", formatDecimal(snapshot.FreezingPointC, 2))},
		{"Overrun estimate", percentString(snapshot.OverrunEstimate, 1)},
		{"Ice fraction @ serve", percentString(snapshot.IceFractionAtServe, 1)},
		{"Viscosity @ serve", fmt.Sprintf("%s Pa·s", formatDecimal(snapshot.ViscosityAtServe, 3))},
		{"Hardness index", formatDecimal(snapshot.HardnessIndex, 2)},
	}
}

func ingredientRows(items []BatchLogIngredient, catalog IngredientCatalog) []ingredientRow {
	if len(items) == 0 {
		return nil
	}
	total := 0.0
	for _, ing := range items {
		total += ing.MassKg
	}
	rows := make([]ingredientRow, 0, len(items))
	for _, ing := range items {
		name := ingredientDisplayName(ing.Key, catalog)
		mass := ing.RawMass
		if mass == "" && ing.MassKg > 0 {
			mass = measurement(ing.MassKg*1000, "g", 0)
		}
		percent := ""
		if total > 0 && ing.MassKg > 0 {
			percent = renderPercent(ing.MassKg / total)
		}
		rows = append(rows, ingredientRow{
			Name:    name,
			Mass:    mass,
			Percent: percent,
		})
	}
	return rows
}

func ingredientDisplayName(key IngredientKey, catalog IngredientCatalog) string {
	if key != "" {
		if inst, ok := catalog.InstanceByKey(key.String()); ok && inst.Definition != nil {
			if inst.Definition.Name != "" {
				return inst.Definition.Name
			}
		}
		return key.String()
	}
	return "ingredient"
}

func recipeSnapshotRows(snapshot *BatchSnapshot) []statRow {
	if snapshot == nil || snapshot.TotalMassKg == 0 {
		return nil
	}
	return []statRow{
		{"Water", percentString(snapshot.WaterPct, 1)},
		{"Total solids", percentString(snapshot.SolidsPct, 1)},
		{"Fat", percentString(snapshot.FatPct, 1)},
		{"Protein", percentString(snapshot.ProteinPct, 1)},
		{"Total sugars", percentString(snapshot.TotalSugarsPct, 1)},
		{"Added sugars", percentString(snapshot.AddedSugarsPct, 1)},
		{"PAC", formatDecimal(snapshot.Sweeteners.TotalPAC, 1)},
		{"POD", formatDecimal(snapshot.Sweeteners.TotalPOD, 1)},
		{"Freezing point", fmt.Sprintf("%s °C", formatDecimal(snapshot.FreezingPointC, 2))},
		{"Overrun", percentString(snapshot.OverrunEstimate, 0)},
	}
}

func recipeMetaRows(snapshot *BatchSnapshot) []statRow {
	if snapshot == nil || snapshot.TotalMassKg == 0 {
		return nil
	}
	return []statRow{
		{"Batch mass", measurement(snapshot.TotalMassKg*1000, "g", 0)},
		{"Mix volume", fmt.Sprintf("%s L", formatDecimal(snapshot.MixVolumeL, 2))},
		{"Pints yield", formatDecimal(snapshot.PintsYield, 1)},
		{"Cost per kg", formatCurrencyPerKg(snapshot.CostPerKg)},
		{"Cost per pint", currencyString(snapshot.CostPerPint)},
	}
}

func formatDecimal(value float64, decimals int) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "—"
	}
	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, value)
}

func currencyString(value float64) string {
	if value <= 0 || math.IsNaN(value) {
		return "—"
	}
	return fmt.Sprintf("$%.2f", value)
}

const baseStyles = `
:root {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  color: #1f2933;
  background: #f7fafc;
}
* {
  box-sizing: border-box;
}
body {
  margin: 0;
  background: #f7fafc;
  color: #1f2933;
  line-height: 1.5;
}
a {
  color: #0369a1;
  text-decoration: none;
}
a:hover {
  text-decoration: underline;
}
.site-header {
  position: sticky;
  top: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  box-shadow: 0 12px 24px -18px rgba(15,23,42,0.45);
  z-index: 10;
}
.logo {
  font-weight: 600;
  letter-spacing: 0.02em;
  color: #0f172a;
}
.site-nav a {
  margin-left: 0.75rem;
  padding: 0.35rem 0.85rem;
  border-radius: 999px;
  color: #475569;
  font-weight: 500;
}
.site-nav a.active {
  background: #e0f2fe;
  color: #035388;
}
main.page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}
.card {
  background: #ffffff;
  border-radius: 0.75rem;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
  border: 1px solid #e5e7eb;
  box-shadow: 0 12px 30px -20px rgba(15,23,42,0.45);
}
.card.issue {
  border-color: #fecaca;
  box-shadow: 0 14px 32px -20px rgba(190,18,60,0.35);
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit,minmax(180px,1fr));
  gap: 1rem;
  margin-bottom: 1.5rem;
}
.summary-card {
  background: #ffffff;
  border-radius: 0.5rem;
  padding: 1rem;
  border: 1px solid #e5e7eb;
  box-shadow: 0 8px 20px -16px rgba(15,23,42,0.35);
}
.summary-label {
  color: #64748b;
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.summary-value {
  font-size: 1.8rem;
  font-weight: 600;
  margin-top: 0.2rem;
}
.grid {
  display: grid;
  gap: 1rem;
}
.grid.multi {
  grid-template-columns: repeat(auto-fit,minmax(260px,1fr));
}
.panel {
  background: #f8fafc;
  border-radius: 0.6rem;
  padding: 1rem;
  border: 1px solid #e5e7eb;
}
.panel h3 {
  margin-top: 0;
  text-transform: uppercase;
  font-size: 0.85rem;
  letter-spacing: 0.08em;
  color: #475569;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.95rem;
}
table td {
  padding: 0.4rem 0;
  border-top: 1px solid #e5e7eb;
  vertical-align: top;
}
table tr:first-child td {
  border-top: none;
}
.status {
  font-weight: 600;
  border-radius: 999px;
  padding: 0.35rem 0.9rem;
}
.status-ok {
  background: #dcfce7;
  color: #166534;
}
.status-failed {
  background: #fee2e2;
  color: #b91c1c;
}
.status-idle {
  background: #fef3c7;
  color: #92400e;
}
.alert {
  background: #fff1f2;
  border: 1px solid #fecdd3;
  border-radius: 0.6rem;
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
}
.meta {
  color: #64748b;
  font-size: 0.9rem;
}
.badge-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0.25rem 0 1rem;
}
.badge {
  background: #e0f2fe;
  color: #0369a1;
  border-radius: 999px;
  padding: 0.2rem 0.7rem;
  font-size: 0.8rem;
}
.fractions-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.fractions-list li {
  display: flex;
  justify-content: space-between;
  padding: 0.25rem 0;
  border-bottom: 1px solid #e5e7eb;
}
.fractions-list li:last-child {
  border-bottom: none;
}
.muted {
  color: #64748b;
}
.notes-list {
  margin: 0.4rem 0 0;
  padding-left: 1.2rem;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.ingredients-table td:first-child {
  font-weight: 600;
  padding-right: 0.5rem;
}
`

const homePageTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Creamery Console</title>
  <style>` + baseStyles + `
  .hero {
    text-align: center;
    padding: 2rem 0 1rem;
  }
  .hero h1 {
    margin-bottom: 0.5rem;
  }
  .hero p {
    color: #64748b;
  }
  .link-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit,minmax(240px,1fr));
    gap: 1rem;
    margin-top: 1.5rem;
  }
  .link-card {
    background: var(--card-bg);
    border-radius: 16px;
    padding: 1.25rem;
    box-shadow: 0 12px 30px rgba(15,23,42,0.08);
  }
  .link-card h2 {
    margin-top: 0;
  }
  .link-card a {
    font-weight: 600;
  }
  </style>
</head>
<body>
  <header class="site-header">
    <div class="logo">Creamery Console</div>
    <nav class="site-nav">
      <a href="/" class="active">Overview</a>
      <a href="/labels">Labels</a>
      <a href="/recipes">Recipes</a>
      <a href="/batchlog">Batch Log</a>
    </nav>
  </header>
  <main class="page">
    <section class="hero">
      <h1>Operational notebook</h1>
      <p>Last refreshed {{formatDateTime .GeneratedAt}} • Label scenarios tracked: {{.LabelCount}}</p>
      <p>Batch log source: <code>{{.BatchLog}}</code></p>
    </section>
    <section class="link-grid">
      <div class="link-card">
        <h2>Label Reconstructions</h2>
        <p>Reverse-engineered ingredient weights, solver diagnostics, and sweetener metrics for every reference label.</p>
        <a href="/labels">Open labels →</a>
      </div>
      <div class="link-card">
        <h2>Recipe Catalog</h2>
        <p>Every logged batch with composition, cost, and tasting notes so you can compare runs quickly.</p>
        <a href="/recipes">Open recipes →</a>
      </div>
      <div class="link-card">
        <h2>Batch Log Dashboard</h2>
        <p>Time-series analytics, ingredient usage, and issue tracking straight from the production log.</p>
        <a href="/batchlog">Open batch log →</a>
      </div>
    </section>
  </main>
</body>
</html>`

const labelsPageTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Label Reconstructions · Creamery Console</title>
  <style>` + baseStyles + `
  .groups-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .groups-list li {
    margin-bottom: 0.6rem;
  }
  .compare table td:nth-child(1) {
    width: 35%;
    color: #64748b;
  }
  .compare table td:nth-child(3) {
    text-align: right;
    color: #64748b;
  }
  </style>
</head>
<body>
  <header class="site-header">
    <div class="logo">Creamery Console</div>
    <nav class="site-nav">
      <a href="/">Overview</a>
      <a href="/labels" class="active">Labels</a>
      <a href="/recipes">Recipes</a>
      <a href="/batchlog">Batch Log</a>
    </nav>
  </header>
  <main class="page">
    <section class="summary-grid">
      <div class="summary-card">
        <p class="summary-label">Scenarios</p>
        <p class="summary-value">{{.CatalogSize}}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Solved</p>
        <p class="summary-value">{{.SolvedCount}}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Failed</p>
        <p class="summary-value">{{.FailedCount}}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Generated</p>
        <p class="summary-value">{{formatDateTime .GeneratedAt}}</p>
      </div>
    </section>
    {{range .Entries}}
    <article class="card {{if .IsError}}issue{{end}}">
      <div class="card-header">
        <div>
          <h2>{{.Name}}</h2>
          <p class="meta">Scenario {{.ID}} • solve {{formatDuration .Duration}}</p>
        </div>
        <span class="status {{.StatusClass}}">{{.Status}}</span>
      </div>
      {{if .Error}}<div class="alert">{{.Error}}</div>{{end}}
      <div class="badge-strip">
        {{if .SolverServing}}<span class="badge">Serving used {{.SolverServing}}</span>{{end}}
        {{if .PintMass}}<span class="badge">Pint mass {{.PintMass}}</span>{{end}}
      </div>
      <div class="grid multi">
        <div class="panel">
          <h3>Label Ingredients</h3>
          {{if .IngredientOrder}}
          <ol>
            {{range .IngredientOrder}}<li>{{.}}</li>{{end}}
          </ol>
          {{else}}<p class="muted">No label order recorded.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Presence Floors</h3>
          {{if .Presence}}
          <ul>
            {{range .Presence}}<li>{{.}}</li>{{end}}
          </ul>
          {{else}}<p class="muted">No minimum ingredients enforced.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Group Constraints</h3>
          {{if .Groups}}
          <ul class="groups-list">
            {{range .Groups}}
            <li>
              <strong>{{.Name}}</strong><br>
              <span class="muted">{{range $i, $m := .Members}}{{if $i}}, {{end}}{{$m}}{{end}}</span>
              {{if .Notes}}
              <ul class="notes-list">{{range .Notes}}<li>{{.}}</li>{{end}}</ul>
              {{end}}
              {{if .EnforceOrder}}<p class="muted">Members locked to label order.</p>{{end}}
            </li>
            {{end}}
          </ul>
          {{else}}<p class="muted">No additional grouping.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Label Facts</h3>
          {{if .LabelFacts}}
          <table>
            {{range .LabelFacts}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}
          </table>
          {{else}}<p class="muted">No label facts recorded.</p>{{end}}
        </div>
        <div class="panel compare">
          <h3>Predicted vs Label</h3>
          {{if .FactComparisons}}
          <table>
            {{range .FactComparisons}}
            <tr>
              <td>{{.Name}}</td>
              <td>{{.Label}} → {{.Pred}}</td>
              <td>{{.Delta}}</td>
            </tr>
            {{end}}
          </table>
          {{else}}<p class="muted">No feasible solution yet.</p>{{end}}
        </div>
      </div>
      <div class="grid multi">
        <div class="panel">
          <h3>Solution Fractions</h3>
          {{if .Fractions}}
          <ul class="fractions-list">
            {{range .Fractions}}<li><span>{{.Name}}</span><span>{{formatPercent .Value}}</span></li>{{end}}
          </ul>
          {{else}}<p class="muted">Solver did not produce a recipe.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Sweetener Analysis</h3>
          {{if .Sweetener}}
          <table>{{range .Sweetener}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}</table>
          {{else}}<p class="muted">Unavailable.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Process Metrics</h3>
          {{if .Metrics}}
          <table>{{range .Metrics}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}</table>
          {{else}}<p class="muted">No process snapshot.</p>{{end}}
        </div>
      </div>
    </article>
    {{end}}
  </main>
</body>
</html>`

const recipesPageTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Recipe Catalog · Creamery Console</title>
  <style>` + baseStyles + `
  .card.recipe h2 {
    margin-bottom: 0.2rem;
  }
  .issues-list {
    margin: 0.4rem 0 0;
    padding-left: 1rem;
  }
  </style>
</head>
<body>
  <header class="site-header">
    <div class="logo">Creamery Console</div>
    <nav class="site-nav">
      <a href="/">Overview</a>
      <a href="/labels">Labels</a>
      <a href="/recipes" class="active">Recipes</a>
      <a href="/batchlog">Batch Log</a>
    </nav>
  </header>
  <main class="page">
    <section class="summary-grid">
      <div class="summary-card">
        <p class="summary-label">Total batches</p>
        <p class="summary-value">{{.Summary.TotalBatches}}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Valid snapshots</p>
        <p class="summary-value">{{.Summary.ValidSnapshots}}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Entries with issues</p>
        <p class="summary-value">{{.Summary.EntriesWithIssues}}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Timeline</p>
        <p class="summary-value">{{formatDate .Summary.EarliestDate}} – {{formatDate .Summary.LatestDate}}</p>
      </div>
    </section>
    {{range .Entries}}
    <article class="card recipe {{if .HasIssues}}issue{{end}}">
      <div class="card-header">
        <div>
          <h2>{{.Label}}</h2>
          <p class="meta">{{if .Date}}Batched {{.Date}}{{else}}Date unavailable{{end}}</p>
        </div>
      </div>
      {{if .HasIssues}}
      <div class="alert">
        <strong>Issues:</strong>
        <ul class="issues-list">{{range .Issues}}<li>{{.}}</li>{{end}}</ul>
      </div>
      {{end}}
      <div class="grid multi">
        <div class="panel">
          <h3>Ingredients</h3>
          {{if .Ingredients}}
          <table class="ingredients-table">
            {{range .Ingredients}}<tr><td>{{.Name}}</td><td>{{.Mass}}</td><td>{{.Percent}}</td></tr>{{end}}
          </table>
          {{else}}<p class="muted">No ingredient weights recorded.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Process Notes</h3>
          {{if .ProcessNotes}}
          <ul class="notes-list">{{range .ProcessNotes}}<li>{{.}}</li>{{end}}</ul>
          {{else}}<p class="muted">None logged.</p>{{end}}
          <h3>Tasting</h3>
          {{if .TastingNotes}}
          <ul class="notes-list">{{range .TastingNotes}}<li>{{.}}</li>{{end}}</ul>
          {{else}}<p class="muted">No tasting notes recorded.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Chemistry Snapshot</h3>
          {{if .SnapshotStats}}
          <table>{{range .SnapshotStats}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}</table>
          {{else}}<p class="muted">Snapshot unavailable.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Sweetener</h3>
          {{if .Sweetener}}
          <table>{{range .Sweetener}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}</table>
          {{else}}<p class="muted">Not computed.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Recipe Fractions</h3>
          {{if .Fractions}}
          <ul class="fractions-list">
            {{range .Fractions}}<li><span>{{.Name}}</span><span>{{formatPercent .Value}}</span></li>{{end}}
          </ul>
          {{else}}<p class="muted">Recipe build missing.</p>{{end}}
        </div>
        <div class="panel">
          <h3>Batch Metrics</h3>
          {{if .Meta}}
          <table>{{range .Meta}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}</table>
          {{else}}<p class="muted">No batch-level metrics.</p>{{end}}
        </div>
      </div>
    </article>
    {{end}}
  </main>
</body>
</html>`
