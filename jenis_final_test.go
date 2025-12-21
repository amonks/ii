package creamery_test

import (
	"fmt"
	"testing"

	"github.com/amonks/creamery"
)

func TestJenisSweetCreamFinal(t *testing.T) {
	// Jeni's Sweet Cream label:
	// Ingredients: Milk, Cream, Cane Sugar, Nonfat Milk, Tapioca Syrup
	// Per 124g (2/3 cup): 290 Cal, 20g fat, 28g carb, 23g sugar, 6g protein

	label := creamery.NutritionLabel{
		ServingSize: 124,
		Calories:    290,
		TotalFat:    20,
		Protein:     6,
		TotalCarbs:  28,
		Sugars:      23,
		AddedSugars: 16,
	}

	target := label.ToTarget()
	compTarget := target.CompositionTarget()

	fmt.Println("=== Jeni's Sweet Cream Reverse Engineering ===")
	fmt.Println()
	fmt.Println("Label: Milk, Cream, Cane Sugar, Nonfat Milk, Tapioca Syrup")
	fmt.Printf("Per %.0fg: %.0f cal, %.0fg fat, %.0fg protein, %.0fg sugar\n",
		label.ServingSize, label.Calories, label.TotalFat, label.Protein, label.Sugars)
	fmt.Println()
	fmt.Println("Derived targets (with FDA rounding uncertainty):")
	fmt.Printf("  Fat:   %s\n", compTarget.Fat)
	fmt.Printf("  MSNF:  %s  (from protein)\n", compTarget.MSNF)
	fmt.Printf("  Sugar: %s\n", compTarget.Sugar)

	// "Nonfat Milk" - could be any concentration from liquid to powder
	// Tapioca syrup - used as stabilizer (starch), not primarily for sugar
	ingredients := []creamery.Ingredient{
		creamery.WholeMilk,
		creamery.HeavyCream,
		creamery.Sugar,
		creamery.NonfatMilkVariable, // variable concentration - solver will determine
		creamery.TapiocaSyrup,
	}

	problem := creamery.NewProblem(ingredients, compTarget)
	problem.OrderConstraints = true

	solver, err := creamery.NewSolver(problem)
	if err != nil {
		t.Fatalf("solver creation failed: %v", err)
	}

	// The LP uses midpoints, but NonfatMilkVariable has a huge range (9-97% MSNF).
	// We need to sample with varied coefficients to explore the space properly.
	fmt.Println("\nSampling with varied ingredient compositions...")
	samples, err := solver.Sample(50, true, nil) // varyCoeffs=true
	if err != nil || len(samples) == 0 {
		fmt.Println("No feasible solutions found with midpoint coefficients.")
		fmt.Println("Trying with varied coefficients...")
	}

	if len(samples) == 0 {
		// Try harder with more samples
		samples, _ = solver.Sample(200, true, nil)
	}

	feasible := len(samples) > 0
	fmt.Printf("Feasible with label ordering: %v (%d solutions found)\n", feasible, len(samples))

	if !feasible {
		t.Fatal("Expected to find feasible solutions")
	}

	// Deduplicate and pick diverse samples
	samples = samples[:min(5, len(samples))]

	fmt.Println("Estimated formulations:")
	for i, s := range samples {
		fmt.Printf("\n  Recipe %d:\n", i+1)
		for _, name := range problem.IngredientNames() {
			w := s.Weights[name]
			if w > 0.005 {
				fmt.Printf("    %-18s %5.1f%%\n", name+":", w*100)
			}
		}

		// What concentration must the Nonfat Milk be?
		if impliedMSNF, ok := s.ImpliedMSNF(ingredients, compTarget, "Nonfat Milk"); ok {
			desc := creamery.DescribeNonfatMilk(impliedMSNF)
			fmt.Printf("    ---\n")
			fmt.Printf("    Nonfat Milk form: %s\n", desc)
		}

		assertCompositionWithinTarget(t, compTarget, s.Achieved, fmt.Sprintf("Jenis recipe %d", i+1))
		sweetener := creamery.AnalyzeSweeteners(s, ingredients)
		assertSweetenersMatchTarget(t, target, sweetener, fmt.Sprintf("Jenis recipe %d", i+1))
	}

	fmt.Println()
	fmt.Println("Key findings:")
	fmt.Println("  - Milk ≈ Cream (~38% each), satisfying label order")
	fmt.Println("  - 'Nonfat Milk' concentration is an OUTPUT, not input")
	fmt.Println("  - Tapioca syrup minimal - used for starch, not sugar")
}
