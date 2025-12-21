package main

import (
	"fmt"
	"log"
	"math"

	"github.com/amonks/creamery"
)

type component struct {
	Key      string
	Fraction float64
	Label    string
}

func main() {
	components := []component{
		{"cream36", 0.432, "Cream (36%)"},
		{"whole_milk", 0.267, "Whole milk (3.25%)"},
		{"skim_milk_powder", 0.112, "Skim milk powder"},
		{"sucrose", 0.190, "Sucrose"},
	}

	const batchMass = 100.0
	pantry := creamery.DetailedIngredientTable()

	keys := make([]string, 0, len(components))
	weights := make([]float64, 0, len(components))
	table := make(map[string]creamery.DetailedIngredient, len(components))
	recipeComponents := make([]creamery.RecipeComponent, 0, len(components))

	for _, comp := range components {
		ing, ok := pantry[comp.Key]
		if !ok {
			log.Fatalf("ingredient %s missing from detailed table", comp.Key)
		}
		mass := comp.Fraction * batchMass
		entry := ing
		keys = append(keys, comp.Key)
		weights = append(weights, mass)
		table[comp.Key] = entry
		recipeComponents = append(recipeComponents, creamery.RecipeComponent{
			Ingredient: &entry,
			MassKg:     mass,
		})
	}

	recipe, err := creamery.NewRecipe(recipeComponents, 0.0)
	if err != nil {
		log.Fatal(err)
	}

	opts := creamery.MixOptions{
		ServeTempC: creamery.DefaultServeTempC(),
		DrawTempC:  creamery.DefaultDrawTempC(),
		ShearRate:  creamery.DefaultShearRate(),
	}

	metrics := creamery.BuildProperties(keys, weights, table, opts)
	if overrun, ok := metrics["overrun_estimate"]; ok {
		if updated, err := recipe.WithOverrun(overrun); err == nil {
			recipe = &updated
		}
	}

	snapshot := creamery.ProductionSettings{
		ServeTempC: opts.ServeTempC,
		DrawTempC:  opts.DrawTempC,
		ShearRate:  opts.ShearRate,
		Metrics:    metrics,
	}
	withSnapshot := recipe.WithMixSnapshot(&snapshot)
	recipe = &withSnapshot

	servingSize, err := recipe.ServingSizeForVolume(creamery.ServingPortionLiters(), opts)
	if err != nil {
		log.Fatal(err)
	}
	facts, err := recipe.NutritionFacts(servingSize, 0)
	if err != nil {
		log.Fatal(err)
	}
	formulation, err := recipe.Formulation()
	if err != nil {
		log.Fatal(err)
	}

	printComponents(components, batchMass)
	fmt.Println("\n=== Recipe Summary ===")
	fmt.Println()
	printRecipe(recipe)

	fmt.Println("\n=== Formulation ===")
	fmt.Println()
	printFormulation(formulation)

	fmt.Println("\n=== Nutrition Facts ===")
	fmt.Println()
	printNutrition(facts)
	fmt.Printf("\n  Serving size used: %.1f g\n", servingSize)

	fmt.Println("\n=== Ingredient Fractions (by mass) ===")
	fractions := recipe.Fractions()
	type pair struct {
		name string
		val  float64
	}
	pairs := make([]pair, 0, len(fractions))
	for k, v := range fractions {
		pairs = append(pairs, pair{k, v})
	}
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].val > pairs[i].val {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	for _, p := range pairs {
		fmt.Printf("  - %-20s%6.2f%%\n", p.name, p.val*100)
	}

	fmt.Println("\n=== Raw Mix Metrics ===")
	keysSorted := make([]pair, 0, len(metrics))
	for k, v := range metrics {
		keysSorted = append(keysSorted, pair{k, v})
	}
	for i := 0; i < len(keysSorted); i++ {
		for j := i + 1; j < len(keysSorted); j++ {
			if keysSorted[j].name < keysSorted[i].name {
				keysSorted[i], keysSorted[j] = keysSorted[j], keysSorted[i]
			}
		}
	}
	for _, p := range keysSorted {
		fmt.Printf("  %-24s: %s\n", p.name, formatValue(p.val))
	}
}

func printComponents(comps []component, batchMass float64) {
	fmt.Println("=== Input Mix ===")
	total := 0.0
	for _, c := range comps {
		total += c.Fraction
	}
	for _, c := range comps {
		weight := c.Fraction * batchMass
		fmt.Printf("  - %-28s%6.2f%% (%6.2f kg)\n", c.Label+" ["+c.Key+"]", c.Fraction*100, weight)
	}
	if !math.IsNaN(total) && math.Abs(total-1.0) > 5e-4 {
		fmt.Printf("  * Note: raw inputs summed to %.2f%% and were renormalized for analysis.\n", total*100)
	}
}

func formatValue(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Sprintf("%v", v)
	}
	abs := math.Abs(v)
	switch {
	case abs == 0:
		return "0"
	case abs >= 1000:
		return fmt.Sprintf("%.2f", v)
	case abs >= 1:
		return fmt.Sprintf("%.4f", v)
	case abs >= 0.01:
		return fmt.Sprintf("%.6f", v)
	default:
		return fmt.Sprintf("%.8f", v)
	}
}

func printRecipe(r *creamery.Recipe) {
	fmt.Println("Recipe:")
	fractions := r.Fractions()
	type entry struct {
		name  string
		value float64
	}
	entries := make([]entry, 0, len(fractions))
	for name, frac := range fractions {
		entries = append(entries, entry{name, frac})
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].value > entries[i].value {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	for _, e := range entries {
		if e.value < 1e-4 {
			continue
		}
		fmt.Printf("  %-20s %6.2f%%\n", e.name, e.value*100)
	}
}

func printFormulation(f creamery.Formulation) {
	fmt.Printf("  Milkfat        %6.2f %%\n", f.MilkfatPct*100)
	fmt.Printf("  SNF            %6.2f %%\n", f.SNFPct*100)
	fmt.Printf("  Water          %6.2f %%\n", f.WaterPct*100)
	fmt.Printf("  Protein        %6.2f %%\n", f.ProteinPct*100)
	fmt.Printf("  Stabilizer     %6.3f %%\n", f.StabilizerPct*100)
	fmt.Printf("  Emulsifier     %6.3f %%\n", f.EmulsifierPct*100)
	for sugar, pct := range f.SugarsPct {
		if pct < 1e-4 {
			continue
		}
		fmt.Printf("  Sugar %-8s %6.2f %%\n", sugar, pct*100)
	}
}

func printNutrition(n creamery.NutritionFacts) {
	fmt.Printf("  Serving size       %.0f g\n", n.ServingSizeGrams)
	fmt.Printf("  Calories           %.0f kcal\n", n.Calories)
	fmt.Printf("  Total fat          %.2f g\n", n.TotalFatGrams)
	fmt.Printf("  Saturated fat      %.2f g\n", n.SaturatedFatGrams)
	fmt.Printf("  Trans fat          %.2f g\n", n.TransFatGrams)
	fmt.Printf("  Total carbs        %.2f g\n", n.TotalCarbGrams)
	fmt.Printf("  Total sugars       %.2f g\n", n.TotalSugarsGrams)
	fmt.Printf("  Added sugars       %.2f g\n", n.AddedSugarsGrams)
	fmt.Printf("  Protein            %.2f g\n", n.ProteinGrams)
	fmt.Printf("  Sodium             %.1f mg\n", n.SodiumMg)
	fmt.Printf("  Cholesterol        %.1f mg\n", n.CholesterolMg)
}
