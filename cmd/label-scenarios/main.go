package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/amonks/creamery"
)

var (
	scenarioFlag = flag.String("scenario", "all", "Comma-separated scenario ids (all, ben, jenis, haagen, brighams, breyers, talenti)")
	htmlOutput   = flag.String("html", "", "Optional path to write an HTML report")
	listSpecs    = flag.Bool("list-specs", false, "List built-in ingredient specs and exit")
)

func main() {
	flag.Parse()

	if *listSpecs {
		printStandardSpecs()
		return
	}

	registry := map[string]func() (*creamery.LabelScenarioResult, error){
		"ben":      creamery.SolveBenAndJerryVanilla,
		"jenis":    creamery.SolveJenisSweetCream,
		"haagen":   creamery.SolveHaagenDazsVanilla,
		"brighams": creamery.SolveBrighamsVanilla,
		"breyers":  creamery.SolveBreyersVanilla,
		"talenti":  creamery.SolveTalentiVanilla,
	}

	var selected []string
	if strings.EqualFold(*scenarioFlag, "all") {
		for key := range registry {
			selected = append(selected, key)
		}
		sort.Strings(selected)
	} else {
		for _, part := range strings.Split(*scenarioFlag, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := registry[part]; !ok {
				log.Fatalf("unknown scenario %q", part)
			}
			selected = append(selected, part)
		}
	}
	if len(selected) == 0 {
		log.Fatal("no scenarios selected")
	}

	results := make([]*creamery.LabelScenarioResult, 0, len(selected))
	for _, key := range selected {
		res, err := registry[key]()
		if err != nil {
			log.Fatalf("scenario %s failed: %v", key, err)
		}
		results = append(results, res)
		printScenario(res)
	}

	if *htmlOutput != "" {
		html, err := creamery.RenderLabelReport(results)
		if err != nil {
			log.Fatalf("unable to render report: %v", err)
		}
		if err := os.WriteFile(*htmlOutput, []byte(html), 0o644); err != nil {
			log.Fatalf("unable to write %s: %v", *htmlOutput, err)
		}
		fmt.Printf("\nWrote HTML report to %s\n", *htmlOutput)
	}
}

func printScenario(result *creamery.LabelScenarioResult) {
	fmt.Printf("\n=== %s ===\n\n", result.Name)
	fmt.Println("Label ingredient order:")
	for _, item := range result.LabelIngredients {
		fmt.Printf("  - %s\n", item)
	}

	fmt.Println("\nSolution weights:")
	for _, spec := range result.Specs {
		w := result.Solution.Weights[spec.ID]
		if w > 1e-4 {
			fmt.Printf("  %s (%s): %.2f%%\n", spec.Name, spec.ID, w*100)
		}
	}

	sweetener := creamery.AnalyzeSweeteners(result.Solution, result.Problem.Specs())
	fmt.Println("\nSweetener analysis:")
	fmt.Printf("  POD: %.1f (%.1f%% sucrose equivalent)\n", sweetener.TotalPOD, sweetener.EquivalentSucrose()*100)
	fmt.Printf("  PAC: %.1f (added %.1f / lactose %.1f)\n", sweetener.TotalPAC, sweetener.AddedSugarPAC, sweetener.LactosePAC)

	fmt.Println("\nMix Recipe:")
	printRecipe(result.Recipe)

	fmt.Println("\nPredicted vs Label Nutrition (per serving):")
	printFactComparison("Calories", result.LabelFacts.Calories, result.PredictedFacts.Calories, "kcal")
	printFactComparison("Total fat", result.LabelFacts.TotalFatGrams, result.PredictedFacts.TotalFatGrams, "g")
	printFactComparison("Saturated fat", result.LabelFacts.SaturatedFatGrams, result.PredictedFacts.SaturatedFatGrams, "g")
	printFactComparison("Total carbs", result.LabelFacts.TotalCarbGrams, result.PredictedFacts.TotalCarbGrams, "g")
	printFactComparison("Total sugars", result.LabelFacts.TotalSugarsGrams, result.PredictedFacts.TotalSugarsGrams, "g")
	printFactComparison("Added sugars", result.LabelFacts.AddedSugarsGrams, result.PredictedFacts.AddedSugarsGrams, "g")
	printFactComparison("Protein", result.LabelFacts.ProteinGrams, result.PredictedFacts.ProteinGrams, "g")

	fmt.Printf("\nServing size used: %.1f g\n", result.ServingSizeGrams)
	fmt.Printf("Pint mass assumption: %.1f g\n", result.PintMassGrams)
}

func printFactComparison(label string, actual, predicted float64, unit string) {
	if actual == 0 && predicted == 0 {
		return
	}
	fmt.Printf("  %-15s %.2f %s (label) / %.2f %s (pred)\n", label, actual, unit, predicted, unit)
}

func printRecipe(r *creamery.Recipe) {
	fmt.Println("  Ingredients:")
	fractions := r.Fractions()
	type entry struct {
		name  string
		value float64
	}
	entries := make([]entry, 0, len(fractions))
	for name, frac := range fractions {
		entries = append(entries, entry{name, frac})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].value > entries[j].value
	})
	for _, e := range entries {
		if e.value < 1e-4 {
			continue
		}
		fmt.Printf("    %-20s %6.2f%%\n", e.name, e.value*100)
	}
	if len(r.Notes) > 0 {
		fmt.Println("  Notes:")
		for _, note := range r.Notes {
			fmt.Printf("    - %s\n", note)
		}
	}
}

func printStandardSpecs() {
	specs := creamery.StandardSpecMap()
	ids := make([]creamery.IngredientID, 0, len(specs))
	for id := range specs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	fmt.Println("Standard ingredient specs:")
	for _, id := range ids {
		spec := specs[id]
		comps := spec.Profile.Components
		added := comps.AddedSugarsInterval()
		fmt.Printf("  %-16s %-24s fat=%5.1f%% protein=%5.1f%% lactose=%5.1f%% added=%5.1f%% water=%5.1f%%\n",
			spec.ID,
			spec.Name,
			comps.Fat.Mid()*100,
			comps.Protein.Mid()*100,
			comps.Lactose.Mid()*100,
			added.Mid()*100,
			comps.Water.Mid()*100,
		)
	}
}
