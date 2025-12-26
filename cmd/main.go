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
	"github.com/amonks/creamery/fdaparser"
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
	case "substitute":
		runSubstitute(args)
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
	logPath := fs.String("log", "batches", "Batch log path")
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
	logPath := fs.String("log", "batches", "Path to batch log file")
	recipesPath := fs.String("recipes", "recipes", "Additional recipe file path")
	_ = fs.Parse(args)

	catalog := creamery.DefaultIngredientCatalog()
	server, err := creamery.NewUnifiedServer(*logPath, *recipesPath, catalog)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Serving console at %s\n", creamery.ServeURL(*addr))
	if err := http.ListenAndServe(*addr, server); err != nil {
		log.Fatal(err)
	}
}

func runSubstitute(args []string) {
	fs := flag.NewFlagSet("substitute", flag.ExitOnError)
	recipePath := fs.String("recipe", "", "Path to recipe .batch file")
	removeIng := fs.String("remove", "", "Ingredient to remove")
	addIngs := fs.String("add", "", "Comma-separated ingredients to add as replacements")
	batchSize := fs.Float64("batch", 1000, "Batch size in grams for output")
	outputPath := fs.String("output", "", "Path to save substituted recipe (optional)")
	_ = fs.Parse(args)

	if *recipePath == "" || *removeIng == "" || *addIngs == "" {
		fmt.Fprintln(os.Stderr, "Usage: substitute --recipe <path> --remove <ing> --add <ing1,ing2,...>")
		os.Exit(1)
	}

	catalog := creamery.DefaultIngredientCatalog()

	// Load the original recipe
	entry, err := creamery.ParseBatchFile(*recipePath)
	if err != nil {
		log.Fatalf("Failed to load recipe: %v", err)
	}

	components, err := entry.Components(catalog)
	if err != nil {
		log.Fatalf("Failed to resolve ingredients: %v", err)
	}

	// Get the original snapshot and formulation
	snapshot, process, err := creamery.BuildProperties(components, creamery.MixOptions{})
	if err != nil {
		log.Fatalf("Failed to build snapshot: %v", err)
	}

	formulation, err := snapshot.FormulationBreakdown()
	if err != nil {
		log.Fatalf("Failed to extract formulation: %v", err)
	}

	// Print original recipe
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("ORIGINAL RECIPE: %s\n", entry.Recipe)
	fmt.Println("=" + strings.Repeat("=", 60))
	origScale := *batchSize / (snapshot.TotalMassKg * 1000)
	fmt.Printf("\nIngredients (scaled to %.0fg batch):\n", *batchSize)
	for _, comp := range components {
		fmt.Printf("  %-20s %6.1fg\n", comp.Ingredient.DisplayName(), comp.MassKg*1000*origScale)
	}
	fmt.Println("\nComposition:")
	fmt.Printf("  Fat:    %.2f%%\n", snapshot.FatPct()*100)
	fmt.Printf("  MSNF:   %.2f%%\n", formulation.SNFPct*100)
	fmt.Printf("  Water:  %.2f%%\n", snapshot.WaterPct()*100)
	fmt.Printf("  Sugars: %.2f%%\n", snapshot.TotalSugarsPct()*100)
	fmt.Println("\nProcess:")
	fmt.Printf("  Freezing point: %.2f°C\n", process.FreezingPointC)
	fmt.Printf("  Overrun est:    %.1f%%\n", process.OverrunEstimate*100)

	// Build new ingredient list: keep everything except the removed one, add replacements
	addList := strings.Split(*addIngs, ",")
	var newSpecs []creamery.Ingredient
	var fixedSpecs []creamery.Ingredient
	var fixedFractions []float64

	for _, comp := range components {
		// Match by ID (key) rather than display name
		var key string
		if comp.Ingredient.Definition != nil {
			key = string(comp.Ingredient.Definition.ID)
		}
		if key == *removeIng {
			continue // skip the removed ingredient
		}
		// Keep this ingredient
		if comp.Ingredient.Definition != nil {
			newSpecs = append(newSpecs, *comp.Ingredient.Definition)
			fixedSpecs = append(fixedSpecs, *comp.Ingredient.Definition)
			fixedFractions = append(fixedFractions, comp.MassKg/snapshot.TotalMassKg)
		}
	}

	// Add replacement ingredients
	for _, ingName := range addList {
		ingName = strings.TrimSpace(ingName)
		lot, ok := catalog.InstanceByKey(ingName)
		if !ok || lot.Definition == nil {
			log.Fatalf("Unknown ingredient: %s", ingName)
		}
		newSpecs = append(newSpecs, *lot.Definition)
	}

	// Create formulation target from original composition with ~10% slack
	tolerance := 0.10 // 10% tolerance on composition targets
	widenInterval := func(iv creamery.Interval) creamery.Interval {
		mid := iv.Mid()
		lo := max(0, mid*(1-tolerance))
		hi := mid * (1 + tolerance)
		return creamery.Range(lo, hi)
	}

	targetComponents := creamery.CompositionRange{
		Fat:          widenInterval(formulation.Components.Fat),
		Protein:      widenInterval(formulation.Components.Protein),
		Lactose:      widenInterval(formulation.Components.Lactose),
		Sucrose:      widenInterval(formulation.Components.Sucrose),
		Glucose:      widenInterval(formulation.Components.Glucose),
		Fructose:     widenInterval(formulation.Components.Fructose),
		Maltodextrin: widenInterval(formulation.Components.Maltodextrin),
		Polyols:      widenInterval(formulation.Components.Polyols),
		Ash:          widenInterval(formulation.Components.Ash),
		OtherSolids:  widenInterval(formulation.Components.OtherSolids),
		MSNF:         widenInterval(formulation.Components.MSNF),
		Water:        widenInterval(formulation.Components.Water),
	}
	target := creamery.FormulationFromFractions(targetComponents)

	// Create and solve the problem
	problem := creamery.NewProblem(newSpecs, target)

	// Fix kept ingredients at their original ratios with some tolerance
	// This preserves the flavor balance while allowing the solver flexibility
	for i, spec := range fixedSpecs {
		frac := fixedFractions[i]
		fixTolerance := 0.02 // 2% tolerance
		lo := max(0, frac*(1-fixTolerance))
		hi := frac * (1 + fixTolerance)
		err := problem.SetWeightBound(spec.ID, lo, hi)
		if err != nil {
			log.Fatalf("Failed to set bounds for %s: %v", spec.Name, err)
		}
	}

	solver, err := creamery.NewSolver(problem)
	if err != nil {
		log.Fatalf("Failed to create solver: %v", err)
	}

	solution, err := solver.FindSolution()
	if err != nil {
		log.Fatalf("Failed to find solution: %v", err)
	}

	// Print substituted recipe
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("SUBSTITUTED RECIPE (replacing %s with %s)\n", *removeIng, *addIngs)
	fmt.Println("=" + strings.Repeat("=", 60))

	fmt.Printf("\nIngredients (for %.0fg batch):\n", *batchSize)
	for _, spec := range newSpecs {
		w := solution.Weights[spec.ID]
		if w > 0.001 {
			fmt.Printf("  %-20s %6.1fg\n", spec.Name, w**batchSize)
		}
	}

	newSnapshot, newProcess, err := solution.Snapshot(creamery.MixOptions{})
	if err != nil {
		log.Fatalf("Failed to compute new snapshot: %v", err)
	}
	newForm, _ := newSnapshot.FormulationBreakdown()

	fmt.Println("\nComposition:")
	fmt.Printf("  Fat:    %.2f%%\n", newSnapshot.FatPct()*100)
	fmt.Printf("  MSNF:   %.2f%%\n", newForm.SNFPct*100)
	fmt.Printf("  Water:  %.2f%%\n", newSnapshot.WaterPct()*100)
	fmt.Printf("  Sugars: %.2f%%\n", newSnapshot.TotalSugarsPct()*100)
	fmt.Println("\nProcess:")
	fmt.Printf("  Freezing point: %.2f°C\n", newProcess.FreezingPointC)
	fmt.Printf("  Overrun est:    %.1f%%\n", newProcess.OverrunEstimate*100)

	// Output as .batch format
	var batchContent strings.Builder
	batchContent.WriteString(fmt.Sprintf("Recipe: %s (substituted)\n\nIngredients:\n", entry.Recipe))
	for _, spec := range newSpecs {
		w := solution.Weights[spec.ID]
		if w > 0.001 {
			batchContent.WriteString(fmt.Sprintf("  %.1fg %s\n", w**batchSize, spec.ID))
		}
	}

	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println("SUBSTITUTED RECIPE (.batch format)")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println()
	fmt.Print(batchContent.String())

	// Save to file if output path specified
	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, []byte(batchContent.String()), 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Printf("\nSaved to: %s\n", *outputPath)
	}
}

func PrintUsage() {
	fmt.Println(`Usage: creamery <command>

Commands:
  labels                         Analyze all label reconstructions
  recipes [--log path --recipes path]   Analyze recipes from batch log plus extras
  notebook [args...]              Run the workflow/notebook sandbox
  serve [--addr --log --recipes]  Start the unified web console
  substitute --recipe <path> --remove <ing> --add <ing1,ing2,...>
                                 Substitute ingredients in a recipe`)
}

func printLabelSummary(label fdaparser.Label) {
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
