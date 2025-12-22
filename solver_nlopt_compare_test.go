package creamery

import (
	"math"
	"testing"
)

func TestNLoptMatchesSimplexForSyntheticProblem(t *testing.T) {
	specs := []IngredientDefinition{
		makeTestIngredient("Rich Base", ComponentFractions{
			Fat:         Point(0.6),
			Sucrose:     Point(0.05),
			OtherSolids: Point(0.05),
			Water:       Point(0.3),
		}),
		makeTestIngredient("Lean Base", ComponentFractions{
			Fat:     Point(0.05),
			Sucrose: Point(0.15),
			Water:   Point(0.8),
		}),
		makeTestIngredient("Dry Solids", ComponentFractions{
			Fat:         Point(0.02),
			OtherSolids: Point(0.98),
		}),
	}

	target := FormulationFromFractions(ComponentFractions{
		Fat:         Range(0.24, 0.26),
		Sucrose:     Range(0.11, 0.13),
		OtherSolids: Range(0.04, 0.06),
	})
	problem := NewProblem(specs, target)
	solver, err := NewSolver(problem)
	if err != nil {
		t.Fatalf("solver creation failed: %v", err)
	}

	lpp := solver.buildLP()
	for j := range specs {
		objective := make([]float64, len(specs))
		objective[j] = 1
		assertSameSolution(t, lpp, objective, solver.opts)

		objectiveNeg := make([]float64, len(specs))
		objectiveNeg[j] = -1
		assertSameSolution(t, lpp, objectiveNeg, solver.opts)
	}
}

func assertSameSolution(t *testing.T, lpp *lpProblem, objective []float64, opts SolverOptions) {
	t.Helper()
	_, weightsNLopt, err := lpp.solveNLopt(objective, opts)
	if err != nil {
		t.Fatalf("nlopt solve failed: %v", err)
	}
	_, weightsSimplex, err := lpp.solveSimplex(objective)
	if err != nil {
		t.Fatalf("simplex solve failed: %v", err)
	}
	if !weightsClose(weightsNLopt, weightsSimplex, 1e-6) {
		t.Fatalf("weights mismatch\ngot:  %v\nwant: %v", weightsNLopt, weightsSimplex)
	}
}

func makeTestIngredient(name string, comps ComponentFractions) IngredientDefinition {
	comps = EnsureWater(comps)
	profile := ConstituentProfile{
		ID:         NewIngredientID(name),
		Name:       name,
		Components: comps,
	}
	return SpecFromProfile(profile)
}

func weightsClose(a, b []float64, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > tol {
			return false
		}
	}
	return true
}
