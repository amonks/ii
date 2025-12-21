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
	target.POD = creamery.Interval{}
	target.PAC = creamery.Interval{}
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

	labelSpecs := []creamery.IngredientSpec{
		creamery.WholeMilk,
		creamery.HeavyCream,
		creamery.Sugar,
		creamery.NonfatMilkVariable, // let the solver decide the concentration
		creamery.TapiocaSyrup,
	}

	labelProblem := creamery.NewProblem(labelSpecs, compTarget)
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
			for _, spec := range labelProblem.Specs() {
				if w := s.Weights[spec.ID]; w > 0.005 {
					fmt.Printf("      %-18s %5.1f%%\n", spec.Name+":", w*100)
				}
			}
			if impliedMSNF, ok := s.ImpliedMSNF(labelSpecs, compTarget, creamery.NonfatMilkVariable.ID); ok {
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
	myCream := creamery.SpecFromComposition("Cream (36%)", creamery.PointComposition(0.36, 0.055, 0, 0))
	myMilk := creamery.SpecFromComposition("Whole Milk (3.25%)", creamery.PointComposition(0.0325, 0.0875, 0, 0))
	myNFDM := creamery.SpecFromComposition("Skim Milk Powder", creamery.PointComposition(0.01, 0.96, 0, 0))

	catalog := creamery.DefaultIngredientCatalog()
	var mySucrose creamery.IngredientSpec
	if inst, ok := catalog.InstanceByKey("sucrose"); ok && inst.Definition != nil {
		mySucrose = *inst.Definition
		mySucrose.Name = "Sucrose"
		mySucrose.ID = creamery.NewIngredientID(mySucrose.Name)
		mySucrose.Profile.Name = mySucrose.Name
		mySucrose.Profile.ID = mySucrose.ID
	}
	var myDextrose creamery.IngredientSpec
	if inst, ok := catalog.InstanceByKey("dextrose"); ok && inst.Definition != nil {
		myDextrose = *inst.Definition
		myDextrose.Name = "Dextrose"
		myDextrose.ID = creamery.NewIngredientID(myDextrose.Name)
		myDextrose.Profile.Name = myDextrose.Name
		myDextrose.Profile.ID = myDextrose.ID
	}
	if mySucrose.ID == "" || myDextrose.ID == "" {
		t.Fatalf("catalog missing required sugar entries")
	}

	fmt.Println("Ingredients on hand:")
	myCreamComp := creamery.CompositionFromProfile(myCream.Profile)
	myMilkComp := creamery.CompositionFromProfile(myMilk.Profile)
	myNFDMComp := creamery.CompositionFromProfile(myNFDM.Profile)
	fmt.Printf("  %-20s Fat: %.1f%%, MSNF: %.1f%%\n", myCream.Name, myCreamComp.Fat.Mid()*100, myCreamComp.MSNF.Mid()*100)
	fmt.Printf("  %-20s Fat: %.2f%%, MSNF: %.2f%%\n", myMilk.Name, myMilkComp.Fat.Mid()*100, myMilkComp.MSNF.Mid()*100)
	fmt.Printf("  %-20s Fat: %.1f%%, MSNF: %.0f%%\n", myNFDM.Name, myNFDMComp.Fat.Mid()*100, myNFDMComp.MSNF.Mid()*100)
	fmt.Printf("  %-20s Sugar: %.0f%%, POD: %.0f, PAC: %.0f\n",
		mySucrose.Name,
		creamery.CompositionFromProfile(mySucrose.Profile).Sugar.Mid()*100,
		mySucrose.Profile.PODInterval().Mid(),
		mySucrose.Profile.PACInterval().Mid())
	fmt.Printf("  %-20s Sugar: %.0f%%, POD: %.0f, PAC: %.0f (less sweet, more softening)\n",
		myDextrose.Name,
		creamery.CompositionFromProfile(myDextrose.Profile).Sugar.Mid()*100,
		myDextrose.Profile.PODInterval().Mid(),
		myDextrose.Profile.PACInterval().Mid())

	specs := []creamery.IngredientSpec{
		myCream,
		myMilk,
		myNFDM,
		mySucrose,
		myDextrose,
	}

	problem := creamery.NewProblem(specs, compTarget)

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
	for _, spec := range specs {
		r := bounds.WeightRanges[spec.ID]
		fmt.Printf("  %-20s %5.1f%% - %5.1f%%\n", spec.Name, r.Lo*100, r.Hi*100)
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
		for _, spec := range specs {
			w := s.Weights[spec.ID]
			if w > 0.001 {
				fmt.Printf("  %-20s %5.1f%%\n", spec.Name, w*100)
			}
		}

		// Print achieved composition
		fmt.Println()
		fmt.Println("Achieved composition:")
		fmt.Printf("  Fat:   %.2f%%  (target: %s)\n", s.Achieved.Fat.Mid()*100, compTarget.Fat)
		fmt.Printf("  MSNF:  %.2f%%  (target: %s)\n", s.Achieved.MSNF.Mid()*100, compTarget.MSNF)
		fmt.Printf("  Sugar: %.2f%%  (target: %s)\n", s.Achieved.Sugar.Mid()*100, compTarget.Sugar)
		fmt.Printf("  Water: %.2f%%\n", s.Achieved.Water().Mid()*100)
		assertCompositionWithinTarget(t, compTarget, s.Achieved, fmt.Sprintf("Recipe %d", i+1))

		// POD/PAC analysis
		sweetener := creamery.AnalyzeSweeteners(s, specs)
		fmt.Println()
		fmt.Println("Sweetener analysis:")
		fmt.Printf("  POD: %.1f (equivalent to %.1f%% sucrose)\n", sweetener.TotalPOD, sweetener.EquivalentSucrose()*100)
		fmt.Printf("  PAC: %.1f → %s\n", sweetener.TotalPAC, sweetener.RelativeSoftness())
		assertSweetenersMatchTarget(t, target, sweetener, fmt.Sprintf("Recipe %d", i+1))

		// Scale to a practical batch size
		batchGrams := 1000.0
		fmt.Println()
		fmt.Printf("For a %.0fg batch:\n", batchGrams)
		for _, spec := range specs {
			w := s.Weights[spec.ID]
			if w > 0.001 {
				grams := w * batchGrams
				fmt.Printf("  %-20s %6.1fg\n", spec.Name, grams)
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
	problem2 := creamery.NewProblem(specs, compTarget)
	problem2.Target.PAC = creamery.Range(28, 32) // target "firm" texture

	solver2, _ := creamery.NewSolver(problem2)
	bounds2, _ := solver2.FindBounds()

	if !bounds2.Feasible {
		fmt.Println("No feasible solution with PAC constraint!")
	} else {
		fmt.Println("With PAC target [28-32]:")
		for _, spec := range specs {
			r := bounds2.WeightRanges[spec.ID]
			if r.Hi > 0.001 {
				fmt.Printf("  %-20s %5.1f%% - %5.1f%%\n", spec.Name, r.Lo*100, r.Hi*100)
			}
		}

		sample2, _ := solver2.FindSolution()
		sweetener2 := creamery.AnalyzeSweeteners(sample2, specs)

		fmt.Println()
		fmt.Println("PAC-optimized recipe (1000g batch):")
		for _, spec := range specs {
			w := sample2.Weights[spec.ID]
			if w > 0.001 {
				fmt.Printf("  %-20s %6.1fg\n", spec.Name, w*1000)
			}
		}
		fmt.Println()
		fmt.Printf("Result: POD=%.1f, PAC=%.1f → %s\n",
			sweetener2.TotalPOD, sweetener2.TotalPAC, sweetener2.RelativeSoftness())
		assertSweetenersMatchTarget(t, problem2.Target, sweetener2, "PAC-optimized recipe")
		assertIntervalContains(t, problem2.Target.PAC, sweetener2.TotalPAC, "PAC-optimized recipe PAC")
	}
}
