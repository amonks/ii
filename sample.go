package creamery

import (
	"math/rand"
)

// Sample generates random feasible solutions by varying both the objective
// direction and (optionally) ingredient compositions within their intervals.
func (s *Solver) Sample(count int, varyCoeffs bool, rng *rand.Rand) ([]*Solution, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	names := s.Problem.IngredientNames()
	n := len(names)
	solutions := make([]*Solution, 0, count)

	for i := 0; i < count; i++ {
		var lpp *lpProblem

		if varyCoeffs {
			// Sample random coefficients within ingredient intervals
			fat := make([]float64, n)
			msnf := make([]float64, n)
			sugar := make([]float64, n)
			other := make([]float64, n)

			for j, ing := range s.Problem.Ingredients {
				fat[j] = sampleInterval(ing.Comp.Fat, rng)
				msnf[j] = sampleInterval(ing.Comp.MSNF, rng)
				sugar[j] = sampleInterval(ing.Comp.Sugar, rng)
				other[j] = sampleInterval(ing.Comp.Other, rng)
			}

			lpp = s.buildLPWithCoeffs(fat, msnf, sugar, other)
		} else {
			lpp = s.buildLP()
		}

		// Random objective to get different corners of the polytope
		objective := make([]float64, n)
		for j := 0; j < n; j++ {
			objective[j] = rng.Float64()*2 - 1 // uniform in [-1, 1]
		}

		_, weights, err := lpp.solve(objective)
		if err != nil {
			continue // skip infeasible samples
		}

		sol := s.weightsToSolutionWithCoeffs(weights, names, lpp)
		solutions = append(solutions, sol)
	}

	return solutions, nil
}

// sampleInterval returns a random value within the interval.
func sampleInterval(i Interval, rng *rand.Rand) float64 {
	if i.IsPoint() {
		return i.Lo
	}
	return i.Lo + rng.Float64()*(i.Hi-i.Lo)
}

// weightsToSolutionWithCoeffs converts weights using specific coefficients.
func (s *Solver) weightsToSolutionWithCoeffs(weights []float64, names []string, lpp *lpProblem) *Solution {
	sol := &Solution{
		Weights: make(map[string]float64),
	}

	for i, w := range weights {
		sol.Weights[names[i]] = w
	}

	// Compute achieved composition using the LP's coefficients
	var fat, msnf, sugar, other float64
	for i, w := range weights {
		fat += w * lpp.fat[i]
		msnf += w * lpp.msnf[i]
		sugar += w * lpp.sugar[i]
		other += w * lpp.other[i]
	}

	sol.Achieved = PointComposition(fat, msnf, sugar, other)
	return sol
}

// ExtremePoints finds solutions at the extreme values of each ingredient.
// This explores the "corners" of the solution space by maximizing and
// minimizing each ingredient's weight.
func (s *Solver) ExtremePoints() ([]*Solution, error) {
	names := s.Problem.IngredientNames()
	n := len(names)
	lpp := s.buildLP()

	solutions := make([]*Solution, 0, 2*n)

	for i := 0; i < n; i++ {
		// Minimize w_i
		minObj := make([]float64, n)
		minObj[i] = 1
		_, weights, err := lpp.solve(minObj)
		if err == nil {
			solutions = append(solutions, s.weightsToSolution(weights, names))
		}

		// Maximize w_i
		maxObj := make([]float64, n)
		maxObj[i] = -1
		_, weights, err = lpp.solve(maxObj)
		if err == nil {
			solutions = append(solutions, s.weightsToSolution(weights, names))
		}
	}

	// Deduplicate solutions that are essentially the same
	return deduplicateSolutions(solutions, 0.001), nil
}

// deduplicateSolutions removes solutions that are nearly identical.
func deduplicateSolutions(solutions []*Solution, tolerance float64) []*Solution {
	if len(solutions) == 0 {
		return solutions
	}

	unique := []*Solution{solutions[0]}

outer:
	for _, sol := range solutions[1:] {
		for _, existing := range unique {
			if solutionsEqual(sol, existing, tolerance) {
				continue outer
			}
		}
		unique = append(unique, sol)
	}

	return unique
}

// solutionsEqual checks if two solutions are approximately equal.
func solutionsEqual(a, b *Solution, tolerance float64) bool {
	if len(a.Weights) != len(b.Weights) {
		return false
	}

	for name, wa := range a.Weights {
		wb, ok := b.Weights[name]
		if !ok {
			return false
		}
		diff := wa - wb
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			return false
		}
	}

	return true
}

// DiverseSamples generates a set of diverse feasible solutions by
// combining extreme points with random samples.
func (s *Solver) DiverseSamples(count int, rng *rand.Rand) ([]*Solution, error) {
	// Start with extreme points
	extremes, err := s.ExtremePoints()
	if err != nil {
		return nil, err
	}

	if len(extremes) >= count {
		return extremes[:count], nil
	}

	// Add random samples to fill out
	remaining := count - len(extremes)
	randoms, err := s.Sample(remaining*2, true, rng) // oversample, will dedupe
	if err != nil {
		return nil, err
	}

	all := append(extremes, randoms...)
	unique := deduplicateSolutions(all, 0.01)

	if len(unique) > count {
		return unique[:count], nil
	}
	return unique, nil
}
