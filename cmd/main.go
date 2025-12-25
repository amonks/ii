package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/amonks/creamery"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		PrintUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "labels":
		runLabels(args)
	case "recipes":
		runRecipes(args)
	case "notebook":
		runNotebook(args)
	case "serve":
		runServe(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", cmd)
		PrintUsage()
		os.Exit(1)
	}
}

func runLabels(args []string) {
	fs := flag.NewFlagSet("labels", flag.ExitOnError)
	_ = fs.Parse(args)

	catalog := creamery.DefaultLabelCatalog()
	report := creamery.AnalyzeLabelCatalog(catalog)
	fmt.Printf("Label analysis (%s)\n", report.GeneratedAt.Format(time.RFC3339))
	for _, entry := range report.Entries {
		label, _ := creamery.FDALabelByKey(entry.Entry.ID)
		fmt.Printf("\n=== %s (%s) ===\n", entry.Entry.Name, entry.Entry.ID)
		printLabelSummary(label)
		if entry.Err != nil {
			fmt.Printf("Status: FAILED — %v\n", entry.Err)
			continue
		}
		if entry.Result == nil {
			fmt.Println("Status: no result produced")
			continue
		}
		fmt.Printf("Status: OK (solve time %s)\n", entry.Duration)
		printLabelResult(entry.Result)
	}
}

func runRecipes(args []string) {
	fs := flag.NewFlagSet("recipes", flag.ExitOnError)
	logPath := fs.String("log", "batchlog", "Batch log path")
	recipesPath := fs.String("recipes", "recipes", "Additional recipe file path")
	_ = fs.Parse(args)

	catalog := creamery.DefaultIngredientCatalog()
	catalogEntries, err := creamery.LoadRecipeCatalogFromFiles([]string{*logPath, *recipesPath}, catalog)
	if err != nil {
		log.Fatal(err)
	}

	analysis := creamery.AnalyzeRecipeCatalog(catalogEntries, catalog)
	fmt.Printf("Recipe analysis (%s)\n", analysis.GeneratedAt.Format(time.RFC3339))
	creamery.PrintBatchLogSummary(os.Stdout, catalogEntries.SourcePath, analysis.Analytics)
	if analysis.Analytics.Summary.TotalBatches > 0 {
		creamery.PrintBatchLogEntries(os.Stdout, analysis.Analytics)
	}
}

func runNotebook(args []string) {
	_ = args // reserved for future notebook flags
	if err := creamery.Notebook(); err != nil {
		log.Fatal(err)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "HTTP listen address")
	logPath := fs.String("log", "batchlog", "Path to batch log file")
	recipesPath := fs.String("recipes", "recipes", "Additional recipe file path")
	_ = fs.Parse(args)

	catalog := creamery.DefaultIngredientCatalog()
	server, err := creamery.NewUnifiedServer(*logPath, *recipesPath, catalog, creamery.DefaultLabelCatalog())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Serving console at %s\n", creamery.ServeURL(*addr))
	if err := http.ListenAndServe(*addr, server); err != nil {
		log.Fatal(err)
	}
}

func PrintUsage() {
	fmt.Println(`Usage: creamery <command>

Commands:
  labels                         Analyze all label reconstructions
  recipes [--log path --recipes path]   Analyze recipes from batch log plus extras
  notebook [args...]              Run the workflow/notebook sandbox
  serve [--addr --log --recipes]  Start the unified web console`)
}

func printLabelSummary(label creamery.Label) {
	if label.Name == "" {
		fmt.Println("Label definition unavailable.")
		return
	}
	facts := label.Facts
	fmt.Printf("Label facts: serve %.1fg | %g kcal | fat %.1fg | carbs %.1fg | sugars %.1fg | protein %.1fg\n",
		facts.ServingSizeGrams, facts.Calories, facts.TotalFatGrams, facts.TotalCarbGrams, facts.TotalSugarsGrams, facts.ProteinGrams)
	if len(label.Ingredients) > 0 {
		names := make([]string, len(label.Ingredients))
		for i, ing := range label.Ingredients {
			names[i] = ing.ID
		}
		fmt.Printf("Ingredients (label order): %s\n", strings.Join(names, ", "))
	}
	if len(label.Groups) > 0 {
		fmt.Println("Group constraints:")
		for _, group := range label.Groups {
			fmt.Printf("  - %s: [%s]\n", group.Name, strings.Join(group.Members, ", "))
		}
	}
}

func printLabelResult(result *creamery.LabelScenarioResult) {
	fmt.Println("\nSolution fractions:")
	fractions := creamery.CombineFractionDisplayAliases(result.Recipe.Fractions())
	type entry struct {
		name  string
		value float64
	}
	entries := make([]entry, 0, len(fractions))
	for name, frac := range fractions {
		if frac > 1e-4 {
			entries = append(entries, entry{name: name, value: frac})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].value > entries[j].value })
	for _, e := range entries {
		fmt.Printf("  %-20s %6.2f%%\n", e.name, e.value*100)
	}

	fmt.Println("\nSweetener analysis:")
	sweetener := creamery.AnalyzeSweeteners(result.Solution, result.Problem.Specs())
	fmt.Printf("  POD: %.1f (%.1f%% sucrose eq)\n", sweetener.TotalPOD, sweetener.EquivalentSucrose()*100)
	fmt.Printf("  PAC: %.1f (added %.1f / lactose %.1f)\n", sweetener.TotalPAC, sweetener.AddedSugarPAC, sweetener.LactosePAC)

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
