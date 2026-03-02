package creamery

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"monks.co/apps/creamery/fdaparser"
)

// UnifiedServer renders labels, recipes, and batch-log analytics from one host.
type UnifiedServer struct {
	catalog        IngredientCatalog
	batchLogPath   string
	recipeLogPath  string
	batchDashboard *BatchLogDashboard

	homeTmpl    *template.Template
	labelsTmpl  *template.Template
	recipesTmpl *template.Template
}

// NewUnifiedServer wires the HTTP handlers needed by the consolidated CLI.
func NewUnifiedServer(batchLogPath, recipeLogPath string, catalog IngredientCatalog) (*UnifiedServer, error) {
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

	home := template.Must(template.New("home").
		Funcs(funcs).
		ParseFS(templateFiles, "base_styles.tmpl", "home.html.tmpl"))
	labels := template.Must(template.New("labels").
		Funcs(funcs).
		ParseFS(templateFiles, "base_styles.tmpl", "labels.html.tmpl"))
	recipes := template.Must(template.New("recipes").
		Funcs(funcs).
		ParseFS(templateFiles, "base_styles.tmpl", "recipes.html.tmpl"))

	return &UnifiedServer{
		catalog:        catalog,
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
		LabelCount:  len(DefaultLabelCatalog()),
	}
	if err := s.homeTmpl.ExecuteTemplate(w, "home", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *UnifiedServer) renderLabels(w http.ResponseWriter, r *http.Request) {
	report := AnalyzeLabelCatalog(DefaultLabelCatalog())
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
		label, _ := FDALabelByKey(entry.Entry.ID)
		var facts NutritionFacts
		if entry.Result != nil {
			facts = entry.Result.LabelFacts
		} else {
			facts = labelFactsFromFDA(label.Facts)
		}
		var fractionByID map[IngredientID]float64
		if entry.Result != nil && entry.Result.Recipe != nil {
			fractionByID = entry.Result.Recipe.batch().FractionsByID()
		}
		card.IngredientOrder = ingredientNamesFromLabel(label)
		card.Presence = presenceNamesFromLabel(label, fractionByID)
		card.Groups = labelGroupsFromFDA(label.Groups, fractionByID)
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
			card.Metrics = processMetricRows(entry.Result.Process)
			card.SolverServing = measurement(entry.Result.ServingSizeGrams, "g", 1)
			card.PintMass = measurement(entry.Result.PintMassGrams, "g", 1)
			data.SolvedCount++
		}
		data.Entries = append(data.Entries, card)
	}
	if err := s.labelsTmpl.ExecuteTemplate(w, "labels", data); err != nil {
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
		}
		if !entry.Raw.Date.IsZero() {
			card.Date = formatDate(entry.Raw.Date)
		}
		if entry.Snapshot != nil {
			card.Composition = recipeCompositionRows(entry.Snapshot)
			card.NutritionFacts = recipeNutritionRows(entry.Snapshot, entry.Process)
			card.Sweetener = sweetenerRows(entry.Snapshot.Sweeteners)
		}
		if entry.Process != nil {
			card.Physics = recipePhysicsRows(entry.Process)
		}
		data.Entries = append(data.Entries, card)
	}
	if err := s.recipesTmpl.ExecuteTemplate(w, "recipes", data); err != nil {
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
	Label          string
	Date           string
	Issues         []string
	HasIssues      bool
	ProcessNotes   []string
	TastingNotes   []string
	Ingredients    []ingredientRow
	Composition    []statRow
	Physics        []statRow
	NutritionFacts []statRow
	Sweetener      []statRow
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

// Helper functions for FDA Label type
func labelFactsFromFDA(facts fdaparser.LabelFacts) NutritionFacts {
	return NutritionFacts{LabelFacts: facts}
}

func ingredientNamesFromLabel(label fdaparser.Label) []string {
	if len(label.Ingredients) == 0 {
		return nil
	}
	names := make([]string, len(label.Ingredients))
	for i, ing := range label.Ingredients {
		names[i] = ing.ID
	}
	return names
}

func presenceNamesFromLabel(label fdaparser.Label, fractions map[IngredientID]float64) []string {
	if len(label.Ingredients) == 0 {
		return nil
	}
	result := make([]string, 0, len(label.Ingredients))
	insertedCream := false
	for _, ing := range label.Ingredients {
		id := NewIngredientID(ing.ID)
		if isCreamComponent(id) {
			if insertedCream {
				continue
			}
			result = append(result, creamAliasLabel(fractions))
			insertedCream = true
			continue
		}
		result = append(result, ing.ID)
	}
	return result
}

func labelGroupsFromFDA(groups []fdaparser.FDAGroup, fractions map[IngredientID]float64) []labelGroupView {
	result := make([]labelGroupView, 0, len(groups))
	for _, group := range groups {
		if len(group.Members) == 0 {
			continue
		}
		view := labelGroupView{
			Name:         group.Name,
			Members:      groupMemberNamesFromFDA(group.Members, fractions),
			EnforceOrder: group.EnforceOrder,
		}
		if len(group.FractionBounds) > 0 {
			notes := make([]string, 0, len(group.FractionBounds))
			for key, bounds := range group.FractionBounds {
				notes = append(notes, describeBounds(key, Interval{Lo: bounds.Lo, Hi: bounds.Hi}))
			}
			sort.Strings(notes)
			view.Notes = notes
		}
		result = append(result, view)
	}
	return result
}

func groupMemberNamesFromFDA(members []string, fractions map[IngredientID]float64) []string {
	if len(members) == 0 {
		return nil
	}
	result := make([]string, 0, len(members))
	insertedCream := false
	for _, member := range members {
		id := NewIngredientID(member)
		if isCreamComponent(id) {
			if insertedCream {
				continue
			}
			result = append(result, creamAliasLabel(fractions))
			insertedCream = true
			continue
		}
		result = append(result, member)
	}
	return result
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
		{"Total POD", formatDecimal(analysis.TotalPOD, 1) + "%"},
		{"Total PAC", formatDecimal(analysis.TotalPAC, 1) + "%"},
		{"Added sugar POD", formatDecimal(analysis.AddedSugarPOD, 1) + "%"},
		{"Lactose POD", formatDecimal(analysis.LactosePOD, 1) + "%"},
		{"Added sugar PAC", formatDecimal(analysis.AddedSugarPAC, 1) + "%"},
		{"Lactose PAC", formatDecimal(analysis.LactosePAC, 1) + "%"},
		{"Sucrose equivalent", percentString(analysis.EquivalentSucrose(), 2)},
		{"Softness", analysis.RelativeSoftness()},
	}
}

func processMetricRows(process ProcessProperties) []statRow {
	return []statRow{
		{"Freezing point", fmt.Sprintf("%s °C", formatDecimal(process.FreezingPointC, 2))},
		{"Overrun estimate", percentString(process.OverrunEstimate, 1)},
		{"Ice fraction @ serve", percentString(process.IceFractionAtServe, 1)},
		{"Viscosity @ serve", fmt.Sprintf("%s Pa·s", formatDecimal(process.ViscosityAtServe, 3))},
		{"Hardness index", formatDecimal(process.HardnessIndex, 2)},
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

// recipeCompositionRows returns composition data from ingredient aggregation.
func recipeCompositionRows(snapshot *BatchSnapshot) []statRow {
	if snapshot == nil || snapshot.TotalMassKg == 0 {
		return nil
	}
	msnfPct := (snapshot.ProteinMassKg + snapshot.Lactose.Mid + snapshot.AshMassKg) / snapshot.TotalMassKg
	emulsifierPct := snapshot.EmulsifierMassKg / snapshot.TotalMassKg

	return []statRow{
		{"Batch mass", measurement(snapshot.TotalMassKg*1000, "g", 0)},
		{"Mix volume", fmt.Sprintf("%s L", formatDecimal(snapshot.MixVolumeL, 2))},
		{"Cost per kg", formatCurrencyPerKg(snapshot.CostPerKg())},
		{"Water", percentString(snapshot.WaterPct(), 1)},
		{"Total solids", percentString(snapshot.SolidsPct(), 1)},
		{"Fat", percentString(snapshot.FatPct(), 1)},
		{"Saturated fat", percentString(snapshot.SaturatedFatPct(), 1)},
		{"Trans fat", percentString(snapshot.TransFatPct(), 2)},
		{"MSNF", percentString(msnfPct, 1)},
		{"Protein", percentString(snapshot.ProteinPct(), 1)},
		{"Lactose", percentString(snapshot.LactosePct(), 1)},
		{"Total sugars", percentString(snapshot.TotalSugarsPct(), 1)},
		{"Added sugars", percentString(snapshot.AddedSugarsPct(), 1)},
		{"Stabilizer", percentString(snapshot.PolymerSolidsPct(), 2)},
		{"Emulsifier", percentString(emulsifierPct, 2)},
		{"Cholesterol", fmt.Sprintf("%s mg/kg", formatDecimal(snapshot.CholesterolMgPerKg(), 0))},
	}
}

// recipePhysicsRows returns process-dependent calculations.
func recipePhysicsRows(process *ProcessProperties) []statRow {
	if process == nil {
		return nil
	}
	return []statRow{
		{"Freezing point", fmt.Sprintf("%s °C", formatDecimal(process.FreezingPointC, 2))},
		{"Ice @ serve", percentString(process.IceFractionAtServe, 1)},
		{"Viscosity @ serve", fmt.Sprintf("%s Pa·s", formatDecimal(process.ViscosityAtServe, 3))},
		{"Overrun", percentString(process.OverrunEstimate, 0)},
		{"Hardness index", formatDecimal(process.HardnessIndex, 2)},
		{"Meltdown index", formatDecimal(process.MeltdownIndex, 2)},
		{"Lactose supersaturation", formatDecimal(process.LactoseSupersaturation, 2)},
		{"Freezer load", fmt.Sprintf("%s kJ", formatDecimal(process.FreezerLoadKJ, 1))},
		{"Pints yield", formatDecimal(process.PintsYield, 1)},
		{"Cost per pint", currencyString(process.CostPerPint)},
	}
}

// recipeNutritionRows derives FDA-style nutrition facts from snapshot data.
// Computes serving mass for 2/3 cup using actual density and overrun.
func recipeNutritionRows(snapshot *BatchSnapshot, process *ProcessProperties) []statRow {
	if snapshot == nil || snapshot.TotalMassKg == 0 || snapshot.MixVolumeL <= 0 {
		return nil
	}
	overrun := 0.0
	if process != nil {
		overrun = process.OverrunEstimate
	}
	const servingVolumeL = 2.0 / 3.0 * 0.236588 // 2/3 cup in liters
	density := snapshot.TotalMassKg / (snapshot.MixVolumeL * (1.0 + overrun))
	servingGrams := density * servingVolumeL * 1000.0
	facts, err := snapshot.NutritionFactsSummary(servingGrams, 0)
	if err != nil {
		return nil
	}
	return []statRow{
		{"Serving size", measurement(facts.ServingSizeGrams, "g", 0)},
		{"Calories", measurement(facts.Calories, "kcal", 0)},
		{"Total fat", measurement(facts.TotalFatGrams, "g", 0)},
		{"Saturated fat", measurement(facts.SaturatedFatGrams, "g", 0)},
		{"Total carbs", measurement(facts.TotalCarbGrams, "g", 0)},
		{"Total sugars", measurement(facts.TotalSugarsGrams, "g", 0)},
		{"Added sugars", measurement(facts.AddedSugarsGrams, "g", 0)},
		{"Protein", measurement(facts.ProteinGrams, "g", 0)},
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
