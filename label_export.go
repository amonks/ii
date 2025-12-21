package creamery

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
)

// RenderLabelReport renders a simple HTML report covering every scenario result.
func RenderLabelReport(results []*LabelScenarioResult) (string, error) {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Ice Cream Label Scenarios</title>`)
	b.WriteString(`<style>
body { font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif; margin: 2rem; background:#fafafa; color:#222; }
h1 { margin-bottom: 1rem; }
section { background:#fff; border-radius:12px; padding:1.5rem; margin-bottom:1.5rem; box-shadow:0 4px 12px rgba(0,0,0,0.05); }
.grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(240px,1fr)); gap:1rem; }
.card { background:#f6f7f9; border-radius:10px; padding:1rem; }
.card h3 { margin-top:0; font-size:1rem; letter-spacing:0.02em; text-transform:uppercase; color:#555; }
ol, ul { margin:0; padding-left:1.25rem; }
table { width:100%; border-collapse:collapse; font-size:0.95rem; }
td { padding:0.2rem 0; }
.recipe-list li { display:flex; justify-content:space-between; }
.tag { display:inline-block; border-radius:999px; background:#e0ecff; color:#0a3d91; padding:0.2rem 0.65rem; font-size:0.85rem; margin-right:0.4rem; }
</style></head><body>`)
	b.WriteString(`<h1>Label Reconstruction Scenarios</h1>`)

	for _, res := range results {
		if res == nil || res.Recipe == nil {
			continue
		}
		formulation, err := res.Recipe.Formulation()
		if err != nil {
			return "", err
		}
		fractions := sortedFractions(res.Recipe.Fractions())
		b.WriteString(`<section>`)
		b.WriteString(fmt.Sprintf(`<h2>%s</h2>`, html.EscapeString(res.Name)))
		b.WriteString(`<div class="grid">`)
		b.WriteString(`<div class="card"><h3>Label Ingredients</h3><ol>`)
		for _, item := range res.LabelIngredients {
			b.WriteString(fmt.Sprintf(`<li>%s</li>`, html.EscapeString(item)))
		}
		b.WriteString(`</ol></div>`)

		b.WriteString(`<div class="card"><h3>Recipe Fractions</h3><ul class="recipe-list">`)
		for _, frac := range fractions {
			b.WriteString(fmt.Sprintf(`<li><span>%s</span><span>%s</span></li>`, html.EscapeString(frac.name), formatPercent(frac.value, 2)))
		}
		b.WriteString(`</ul></div>`)

		b.WriteString(`<div class="card"><h3>Formulation</h3><table>`)
		b.WriteString(fmt.Sprintf(`<tr><td>Milkfat</td><td>%s</td></tr>`, formatPercent(formulation.MilkfatPct, 2)))
		b.WriteString(fmt.Sprintf(`<tr><td>SNF</td><td>%s</td></tr>`, formatPercent(formulation.SNFPct, 2)))
		b.WriteString(fmt.Sprintf(`<tr><td>Water</td><td>%s</td></tr>`, formatPercent(formulation.WaterPct, 2)))
		b.WriteString(fmt.Sprintf(`<tr><td>Protein</td><td>%s</td></tr>`, formatPercent(formulation.ProteinPct, 2)))
		b.WriteString(fmt.Sprintf(`<tr><td>Stabilizer</td><td>%s</td></tr>`, formatPercent(formulation.StabilizerPct, 3)))
		b.WriteString(fmt.Sprintf(`<tr><td>Emulsifier</td><td>%s</td></tr>`, formatPercent(formulation.EmulsifierPct, 3)))
		b.WriteString(`</table></div>`)

		b.WriteString(`<div class="card"><h3>Nutrition (Label / Predicted)</h3><table>`)
		for _, row := range []struct {
			label    string
			actual   float64
			pred     float64
			unit     string
			decimals int
		}{
			{"Calories", res.LabelFacts.Calories, res.PredictedFacts.Calories, "kcal", 0},
			{"Total fat", res.LabelFacts.TotalFatGrams, res.PredictedFacts.TotalFatGrams, "g", 1},
			{"Saturated fat", res.LabelFacts.SaturatedFatGrams, res.PredictedFacts.SaturatedFatGrams, "g", 1},
			{"Total carbs", res.LabelFacts.TotalCarbGrams, res.PredictedFacts.TotalCarbGrams, "g", 1},
			{"Total sugars", res.LabelFacts.TotalSugarsGrams, res.PredictedFacts.TotalSugarsGrams, "g", 1},
			{"Added sugars", res.LabelFacts.AddedSugarsGrams, res.PredictedFacts.AddedSugarsGrams, "g", 1},
			{"Protein", res.LabelFacts.ProteinGrams, res.PredictedFacts.ProteinGrams, "g", 1},
		} {
			if row.actual == 0 && row.pred == 0 {
				continue
			}
			b.WriteString(`<tr><td>`)
			b.WriteString(html.EscapeString(row.label))
			b.WriteString(`</td><td>`)
			b.WriteString(fmt.Sprintf("%s / %s", formatValue(row.actual, row.unit, row.decimals), formatValue(row.pred, row.unit, row.decimals)))
			b.WriteString(`</td></tr>`)
		}
		b.WriteString(`</table></div>`)

		if res.Metrics.TotalMassKg > 0 {
			b.WriteString(`<div class="card"><h3>Process Metrics</h3><table>`)
			b.WriteString(fmt.Sprintf(`<tr><td>Freezing point</td><td>%s</td></tr>`, formatNumber(res.Metrics.FreezingPointC, 2, "°C")))
			b.WriteString(fmt.Sprintf(`<tr><td>Overrun estimate</td><td>%s</td></tr>`, formatPercent(res.Metrics.OverrunEstimate, 1)))
			b.WriteString(fmt.Sprintf(`<tr><td>Viscosity @ serve</td><td>%s</td></tr>`, formatNumber(res.Metrics.ViscosityAtServe, 4, "Pa·s")))
			b.WriteString(fmt.Sprintf(`<tr><td>Hardness index</td><td>%s</td></tr>`, formatNumber(res.Metrics.HardnessIndex, 2, "")))
			b.WriteString(`</table></div>`)
		}

		b.WriteString(`</div>`)

		if res.Solution != nil && res.Problem != nil {
			sweetener := AnalyzeSweeteners(res.Solution, res.Problem.Specs)
			if sweetener.TotalPOD > 0 || sweetener.TotalPAC > 0 {
				b.WriteString(`<div class="grid">`)
				b.WriteString(`<div class="card"><h3>Sweetener Analysis</h3><table>`)
				b.WriteString(fmt.Sprintf(`<tr><td>Total POD</td><td>%.1f</td></tr>`, sweetener.TotalPOD))
				b.WriteString(fmt.Sprintf(`<tr><td>Total PAC</td><td>%.1f</td></tr>`, sweetener.TotalPAC))
				b.WriteString(fmt.Sprintf(`<tr><td>Added sugars POD</td><td>%.1f</td></tr>`, sweetener.AddedSugarPOD))
				b.WriteString(fmt.Sprintf(`<tr><td>Lactose POD</td><td>%.1f</td></tr>`, sweetener.LactosePOD))
				b.WriteString(fmt.Sprintf(`<tr><td>Added sugars PAC</td><td>%.1f</td></tr>`, sweetener.AddedSugarPAC))
				b.WriteString(fmt.Sprintf(`<tr><td>Lactose PAC</td><td>%.1f</td></tr>`, sweetener.LactosePAC))
				b.WriteString(`</table></div>`)

				b.WriteString(`<div class="card"><h3>IngredientSpec IDs</h3><p>`)
				for _, spec := range res.Specs {
					b.WriteString(fmt.Sprintf(`<span class="tag">%s</span>`, html.EscapeString(spec.ID.String())))
				}
				b.WriteString(`</p></div>`)
				b.WriteString(`</div>`)
			}
		}

		b.WriteString(fmt.Sprintf(`<p><span class="tag">Serving size %.1f g</span><span class="tag">Pint mass %.1f g</span></p>`, res.ServingSizeGrams, res.PintMassGrams))
		b.WriteString(`</section>`)
	}

	b.WriteString(`</body></html>`)
	return b.String(), nil
}

type fractionEntry struct {
	name  string
	value float64
}

func sortedFractions(fractions map[string]float64) []fractionEntry {
	entries := make([]fractionEntry, 0, len(fractions))
	for name, value := range fractions {
		if value < 1e-4 {
			continue
		}
		entries = append(entries, fractionEntry{name: name, value: value})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].value > entries[j].value
	})
	return entries
}

func formatPercent(value float64, decimals int) string {
	if math.IsNaN(value) {
		return "--"
	}
	format := fmt.Sprintf("%%.%df%%%%", decimals)
	return fmt.Sprintf(format, value*100)
}

func formatValue(value float64, unit string, decimals int) string {
	if math.IsNaN(value) {
		return "--"
	}
	format := fmt.Sprintf("%%.%df %%s", decimals)
	return fmt.Sprintf(format, value, unit)
}

func formatNumber(value float64, decimals int, unit string) string {
	if math.IsNaN(value) {
		return "--"
	}
	format := fmt.Sprintf("%%.%df", decimals)
	if unit != "" {
		format += " %s"
		return fmt.Sprintf(format, value, unit)
	}
	return fmt.Sprintf(format, value)
}
