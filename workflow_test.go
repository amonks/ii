package creamery_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/amonks/creamery"
)

func TestFullWorkflow(t *testing.T) {
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println("FULL WORKFLOW: Label → Formulation → Recipe")
	fmt.Println("=" + strings.Repeat("=", 60))

	// =========================================================================
	// WORKFLOW 1: Reverse-engineer formulation from Jeni's label
	// =========================================================================

	fmt.Println("\n## STEP 1: Analyze the label")
	fmt.Println()

	label := creamery.NutritionLabel{
		ServingSize: 124,
		Calories:    290,
		TotalFat:    20,
		Protein:     6,
		TotalCarbs:  28,
		Sugars:      23,
		AddedSugars: 16,
	}

	fmt.Println("Jeni's Sweet Cream label:")
	fmt.Println("  Ingredients: Milk, Cream, Cane Sugar, Nonfat Milk, Tapioca Syrup")
	fmt.Printf("  Per %.0fg: %.0fg fat, %.0fg protein, %.0fg sugar\n",
		label.ServingSize, label.TotalFat, label.Protein, label.Sugars)

	// Derive target composition from label
	target := label.ToTarget()
	compTarget := target.CompositionTarget()

	fmt.Println()
	fmt.Println("Derived formulation targets (with FDA rounding uncertainty):")
	fmt.Printf("  Fat:   %s\n", compTarget.Fat)
	fmt.Printf("  MSNF:  %s\n", compTarget.MSNF)
	fmt.Printf("  Sugar: %s\n", compTarget.Sugar)
	fmt.Printf("  Water: %s (derived)\n", compTarget.Water())

	fmt.Println()
	fmt.Println("### Interpreting the ingredient list")
	fmt.Println("Using the label's ingredient order to constrain possible formulations:")

	labelIngredients := []creamery.Ingredient{
		creamery.WholeMilk,
		creamery.HeavyCream,
		creamery.Sugar,
		creamery.NonfatMilkVariable, // let the solver decide the concentration
		creamery.TapiocaSyrup,
	}

	labelProblem := creamery.NewProblem(labelIngredients, compTarget)
	labelProblem.OrderConstraints = true

	labelSolver, err := creamery.NewSolver(labelProblem)
	if err != nil {
		t.Fatalf("label solver creation failed: %v", err)
	}

	labelSamples, err := labelSolver.Sample(50, true, nil)
	if err != nil {
		t.Fatalf("label sampling failed: %v", err)
	}
	if len(labelSamples) == 0 {
		fmt.Println("  No formulation satisfies both the label macros and ingredient order.")
	} else {
		fmt.Println("  Candidate formulations:")
		limit := min(2, len(labelSamples))
		for i := 0; i < limit; i++ {
			s := labelSamples[i]
			fmt.Printf("    Option %d:\n", i+1)
			for _, name := range labelProblem.IngredientNames() {
				if w := s.Weights[name]; w > 0.005 {
					fmt.Printf("      %-18s %5.1f%%\n", name+":", w*100)
				}
			}
			if impliedMSNF, ok := s.ImpliedMSNF(labelIngredients, compTarget, "Nonfat Milk"); ok {
				desc := creamery.DescribeNonfatMilk(impliedMSNF)
				fmt.Printf("      └─ Nonfat milk form: %s\n", desc)
			}
		}
	}

	// =========================================================================
	// WORKFLOW 2: Create a recipe with known ingredients
	// =========================================================================

	fmt.Println()
	fmt.Println("## STEP 2: Create recipe with ingredients on hand")
	fmt.Println()

	// Define our specific ingredients with KNOWN compositions (point values)
	myCream := creamery.Ingredient{
		Name: "Cream (36%)",
		Comp: creamery.PointComposition(0.36, 0.055, 0, 0), // 36% fat, 5.5% MSNF
	}

	myMilk := creamery.Ingredient{
		Name: "Whole Milk (3.25%)",
		Comp: creamery.PointComposition(0.0325, 0.0875, 0, 0), // 3.25% fat, 8.75% MSNF
	}

	myNFDM := creamery.Ingredient{
		Name: "Skim Milk Powder",
		Comp: creamery.PointComposition(0.01, 0.96, 0, 0), // 1% fat, 96% MSNF
	}

	mySucroseBatch := creamery.IngredientBatchTable()["sucrose"]
	mySucroseBatch.Name = "Sucrose"
	mySucrose := mySucroseBatch.ToSpec().LegacyIngredient()

	myDextroseBatch := creamery.IngredientBatchTable()["dextrose"]
	myDextroseBatch.Name = "Dextrose"
	myDextrose := myDextroseBatch.ToSpec().LegacyIngredient()

	fmt.Println("Ingredients on hand:")
	fmt.Printf("  %-20s Fat: %.1f%%, MSNF: %.1f%%\n", myCream.Name, myCream.Comp.Fat.Mid()*100, myCream.Comp.MSNF.Mid()*100)
	fmt.Printf("  %-20s Fat: %.2f%%, MSNF: %.2f%%\n", myMilk.Name, myMilk.Comp.Fat.Mid()*100, myMilk.Comp.MSNF.Mid()*100)
	fmt.Printf("  %-20s Fat: %.1f%%, MSNF: %.0f%%\n", myNFDM.Name, myNFDM.Comp.Fat.Mid()*100, myNFDM.Comp.MSNF.Mid()*100)
	fmt.Printf("  %-20s Sugar: %.0f%%, POD: %.0f, PAC: %.0f\n",
		mySucrose.Name,
		mySucrose.Comp.Sugar.Mid()*100,
		mySucrose.Profile.PODInterval().Mid(),
		mySucrose.Profile.PACInterval().Mid())
	fmt.Printf("  %-20s Sugar: %.0f%%, POD: %.0f, PAC: %.0f (less sweet, more softening)\n",
		myDextrose.Name,
		myDextrose.Comp.Sugar.Mid()*100,
		myDextrose.Profile.PODInterval().Mid(),
		myDextrose.Profile.PACInterval().Mid())

	ingredients := []creamery.Ingredient{
		myCream,
		myMilk,
		myNFDM,
		mySucrose,
		myDextrose,
	}

	problem := creamery.NewProblem(ingredients, compTarget)

	solver, err := creamery.NewSolver(problem)
	if err != nil {
		t.Fatalf("solver creation failed: %v", err)
	}

	// Find bounds on each ingredient
	bounds, err := solver.FindBounds()
	if err != nil {
		t.Fatalf("bounds failed: %v", err)
	}

	fmt.Println()
	if !bounds.Feasible {
		fmt.Println("WARNING: No feasible recipe with these ingredients!")
		fmt.Println("The target formulation cannot be achieved.")
		return
	}

	fmt.Println("Feasible ingredient ranges:")
	for _, ing := range ingredients {
		r := bounds.WeightRanges[ing.Name]
		fmt.Printf("  %-20s %5.1f%% - %5.1f%%\n", ing.Name, r.Lo*100, r.Hi*100)
	}

	// Get diverse sample recipes
	samples, _ := solver.DiverseSamples(3, nil)

	fmt.Println()
	fmt.Println("## STEP 3: Sample recipes")

	for i, s := range samples {
		fmt.Printf("\n### Recipe %d\n", i+1)
		fmt.Println()

		// Print recipe
		fmt.Println("Ingredients:")
		for _, ing := range ingredients {
			w := s.Weights[ing.Name]
			if w > 0.001 {
				fmt.Printf("  %-20s %5.1f%%\n", ing.Name, w*100)
			}
		}

		// Print achieved composition
		fmt.Println()
		fmt.Println("Achieved composition:")
		fmt.Printf("  Fat:   %.2f%%  (target: %s)\n", s.Achieved.Fat.Mid()*100, compTarget.Fat)
		fmt.Printf("  MSNF:  %.2f%%  (target: %s)\n", s.Achieved.MSNF.Mid()*100, compTarget.MSNF)
		fmt.Printf("  Sugar: %.2f%%  (target: %s)\n", s.Achieved.Sugar.Mid()*100, compTarget.Sugar)
		fmt.Printf("  Water: %.2f%%\n", s.Achieved.Water().Mid()*100)

		// POD/PAC analysis
		sweetener := creamery.AnalyzeSweeteners(s, ingredients)
		fmt.Println()
		fmt.Println("Sweetener analysis:")
		fmt.Printf("  POD: %.1f (equivalent to %.1f%% sucrose)\n", sweetener.TotalPOD, sweetener.EquivalentSucrose()*100)
		fmt.Printf("  PAC: %.1f → %s\n", sweetener.TotalPAC, sweetener.RelativeSoftness())

		// Scale to a practical batch size
		batchGrams := 1000.0
		fmt.Println()
		fmt.Printf("For a %.0fg batch:\n", batchGrams)
		for _, ing := range ingredients {
			w := s.Weights[ing.Name]
			if w > 0.001 {
				grams := w * batchGrams
				fmt.Printf("  %-20s %6.1fg\n", ing.Name, grams)
			}
		}
	}

	// =========================================================================
	// Show how to use PAC to target texture
	// =========================================================================

	fmt.Println()
	fmt.Println("## STEP 4: Target texture with PAC constraint")
	fmt.Println()
	fmt.Println("Problem: sucrose alone → PAC ~24 (hard)")
	fmt.Println("         dextrose alone → PAC ~41 (too soft)")
	fmt.Println("Solution: blend them to target PAC 28-32 (firm, scoopable)")
	fmt.Println()

	// Create a new problem with PAC target
	problem2 := creamery.NewProblem(ingredients, compTarget)
	problem2.TargetPAC = creamery.Range(28, 32) // target "firm" texture

	solver2, _ := creamery.NewSolver(problem2)
	bounds2, _ := solver2.FindBounds()

	if !bounds2.Feasible {
		fmt.Println("No feasible solution with PAC constraint!")
	} else {
		fmt.Println("With PAC target [28-32]:")
		for _, ing := range ingredients {
			r := bounds2.WeightRanges[ing.Name]
			if r.Hi > 0.001 {
				fmt.Printf("  %-20s %5.1f%% - %5.1f%%\n", ing.Name, r.Lo*100, r.Hi*100)
			}
		}

		sample2, _ := solver2.FindSolution()
		sweetener2 := creamery.AnalyzeSweeteners(sample2, ingredients)

		fmt.Println()
		fmt.Println("PAC-optimized recipe (1000g batch):")
		for _, ing := range ingredients {
			w := sample2.Weights[ing.Name]
			if w > 0.001 {
				fmt.Printf("  %-20s %6.1fg\n", ing.Name, w*1000)
			}
		}
		fmt.Println()
		fmt.Printf("Result: POD=%.1f, PAC=%.1f → %s\n",
			sweetener2.TotalPOD, sweetener2.TotalPAC, sweetener2.RelativeSoftness())
	}
}
