package creamery_test

import (
	"fmt"
	"testing"

	"github.com/amonks/creamery"
)

// TestWorkflow2_FormulationToRecipe demonstrates finding recipes to hit a target formulation.
func TestWorkflow2_FormulationToRecipe(t *testing.T) {
	// Target: a classic vanilla ice cream
	// ~16% fat, ~10% MSNF, ~14% sugar, ~0.3% stabilizer
	target := creamery.FormulationFromFractions(creamery.ComponentFractions{
		Fat:         creamery.Range(0.15, 0.17),
		MSNF:        creamery.Range(0.09, 0.11),
		Sucrose:     creamery.Range(0.13, 0.15),
		OtherSolids: creamery.Range(0, 0.01),
	})

	// Available ingredients
	specs := []creamery.Ingredient{
		creamery.HeavyCream,
		creamery.WholeMilk,
		creamery.Sugar,
		creamery.NonfatDryMilk,
	}

	problem := creamery.NewProblem(specs, target)

	solver, err := creamery.NewSolver(problem)
	if err != nil {
		t.Fatalf("failed to create solver: %v", err)
	}

	// Check feasibility
	feasible, err := solver.Feasible()
	if err != nil {
		t.Fatalf("feasibility check failed: %v", err)
	}
	if !feasible {
		t.Fatal("expected problem to be feasible")
	}

	// Find bounds
	bounds, err := solver.FindBounds()
	if err != nil {
		t.Fatalf("failed to find bounds: %v", err)
	}

	fmt.Println("=== Workflow 2: Formulation -> Recipe ===")
	fmt.Printf("Target: Fat %s, MSNF %s, Sugar %s, Other %s\n",
		target.FatInterval(), target.MSNFInterval(), target.AddedSugarsInterval(), target.OtherSolidsInterval())
	fmt.Println()
	fmt.Println(bounds)

	// Find a solution
	solution, err := solver.FindSolution()
	if err != nil {
		t.Fatalf("failed to find solution: %v", err)
	}
	fmt.Println("Example solution:")
	fmt.Println(solution)

	// Show just the best sample from the distribution.
	bestSample, err := solver.Optimize(nil)
	if err != nil {
		t.Fatalf("failed to get best sample: %v", err)
	}
	fmt.Println("\nBest sample (viscosity preference):")
	for id, w := range bestSample.Weights {
		if w > 0.01 {
			name := bestSample.Names[id]
			if name == "" {
				name = id.String()
			}
			fmt.Printf("  %s: %.1f%%\n", name, w*100)
		}
	}
}

// TestWorkflow1_LabelToFormulation demonstrates reverse-engineering from a label.
func TestWorkflow1_LabelToFormulation(t *testing.T) {
	// Hypothetical FDA label for a premium vanilla ice cream
	// Serving size: 100g
	// Calories: 220
	// Total Fat: 14g
	// Protein: 4g
	// Total Carbs: 22g
	// Sugars: 20g

	label := creamery.NutritionLabel{
		ServingSize: 100,
		Calories:    220,
		TotalFat:    14,
		Protein:     4,
		TotalCarbs:  22,
		Sugars:      20,
		AddedSugars: 20,
	}

	target := label.ToTarget()

	fmt.Println("=== Workflow 1: Label -> Formulation ===")
	fmt.Printf("Label: %dg serving, %d cal, %dg fat, %dg protein, %dg carbs, %dg sugar\n",
		int(label.ServingSize), int(label.Calories), int(label.TotalFat),
		int(label.Protein), int(label.TotalCarbs), int(label.Sugars))
	fmt.Printf("Derived target: %s\n\n", target)

	// Ingredients in label order (descending by weight)
	specs := []creamery.Ingredient{
		creamery.HeavyCream,
		creamery.WholeMilk,
		creamery.Sugar,
		creamery.EggYolks,
	}

	problem := creamery.NewProblem(specs, target)
	problem.OrderConstraints = true // enforce label ordering

	solver, err := creamery.NewSolver(problem)
	if err != nil {
		t.Fatalf("failed to create solver: %v", err)
	}

	feasible, err := solver.Feasible()
	if err != nil {
		t.Fatalf("feasibility check failed: %v", err)
	}

	if !feasible {
		fmt.Println("No feasible solution with these ingredients and ordering constraints.")
		fmt.Println("This could mean the label doesn't match expected ingredient compositions,")
		fmt.Println("or there are additional/different ingredients involved.")
		return
	}

	bounds, err := solver.FindBounds()
	if err != nil {
		t.Fatalf("failed to find bounds: %v", err)
	}
	fmt.Println(bounds)

	bestFormulation, err := solver.Optimize(nil)
	if err != nil {
		t.Fatalf("failed to sample: %v", err)
	}

	fmt.Println("Best formulation (viscosity preference):")
	for id, w := range bestFormulation.Weights {
		if w > 0.01 {
			name := bestFormulation.Names[id]
			if name == "" {
				name = id.String()
			}
			fmt.Printf("  %s: %.1f%%\n", name, w*100)
		}
	}
	fmt.Printf("  Achieved: %s\n", bestFormulation.Achieved)
}

// TestWithTighterConstraints shows iterative refinement.
func TestWithTighterConstraints(t *testing.T) {
	target := creamery.FormulationFromFractions(creamery.ComponentFractions{
		Fat:         creamery.Range(0.15, 0.17),
		MSNF:        creamery.Range(0.09, 0.11),
		Sucrose:     creamery.Range(0.13, 0.15),
		OtherSolids: creamery.Range(0, 0.01),
	})

	specs2 := []creamery.Ingredient{
		creamery.HeavyCream,
		creamery.WholeMilk,
		creamery.Sugar,
		creamery.NonfatDryMilk,
	}

	problem := creamery.NewProblem(specs2, target)

	// First pass: see what's possible
	solver, _ := creamery.NewSolver(problem)
	bounds1, _ := solver.FindBounds()

	fmt.Println("=== Iterative Refinement ===")
	fmt.Println("Initial bounds:")
	fmt.Println(bounds1)

	// User decides: "I want at least 35% cream for richness"
	if err := problem.SetMinWeightByName(creamery.HeavyCream.Name, 0.35); err != nil {
		t.Fatalf("failed to set min weight: %v", err)
	}

	solver2, _ := creamery.NewSolver(problem)
	bounds2, _ := solver2.FindBounds()

	fmt.Println("After requiring >= 35% cream:")
	fmt.Println(bounds2)

	// "And I want to minimize milk powder"
	if err := problem.SetMaxWeightByName(creamery.NonfatDryMilk.Name, 0.05); err != nil {
		t.Fatalf("failed to set max weight: %v", err)
	}

	solver3, _ := creamery.NewSolver(problem)
	bounds3, err := solver3.FindBounds()
	if err != nil {
		t.Fatalf("solver failed: %v", err)
	}

	fmt.Println("After capping milk powder at 5%:")
	if !bounds3.Feasible {
		fmt.Println("No feasible solution! Need to relax constraints.")
	} else {
		fmt.Println(bounds3)
	}
}
