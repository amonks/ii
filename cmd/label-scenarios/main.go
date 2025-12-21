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
)

func main() {
	flag.Parse()

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
	for _, ing := range result.Problem.Ingredients {
		w := result.Solution.Weights[ing.Name]
		if w > 1e-4 {
			fmt.Printf("  %s: %.2f%%\n", ing.Name, w*100)
		}
	}

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
