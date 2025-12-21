package creamery

import (
	"math"
	"math/rand"
)

// Sample generates random feasible solutions by varying both the objective
// direction and (optionally) ingredient compositions within their intervals.
func (s *Solver) Sample(count int, varyCoeffs bool, rng *rand.Rand) ([]*Solution, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	ids := s.Problem.IngredientIDs()
	names := s.Problem.IngredientNames()
	n := len(names)
	solutions := make([]*Solution, 0, count)

	for i := 0; i < count; i++ {
		var lpp *lpProblem

		if varyCoeffs {
			coeffs := newCoefficientSet(n)
			for j := range s.Problem.Specs {
				profile := s.Problem.profileForIndex(j)
				point := sampleProfilePoint(profile, rng)
				coeffs.fat[j] = point.fat
				coeffs.msnf[j] = point.msnf
				coeffs.sugar[j] = point.addedSugar
				coeffs.other[j] = point.other
				coeffs.protein[j] = point.protein
				coeffs.lactose[j] = point.lactose
				coeffs.totalSugar[j] = point.totalSugar
				coeffs.water[j] = point.water
				coeffs.pod[j] = point.pod
				coeffs.pac[j] = point.pac
			}

			lpp = s.buildLPWithCoeffs(coeffs)
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

		sol := s.weightsToSolutionWithCoeffs(weights, ids, names, lpp)
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

type profilePoint struct {
	fat        float64
	msnf       float64
	addedSugar float64
	other      float64
	protein    float64
	lactose    float64
	totalSugar float64
	water      float64
	pod        float64
	pac        float64
}

func sampleProfilePoint(profile ConstituentProfile, rng *rand.Rand) profilePoint {
	comps := profile.Components
	point := profilePoint{
		fat:     sampleInterval(comps.Fat, rng),
		protein: sampleInterval(comps.Protein, rng),
		lactose: sampleInterval(comps.Lactose, rng),
		other:   sampleInterval(comps.OtherSolids, rng),
		water:   sampleInterval(comps.Water, rng),
	}
	ash := sampleInterval(comps.Ash, rng)
	msnf := sampleInterval(comps.MSNF, rng)
	if msnf == 0 {
		msnf = point.protein + point.lactose + ash
	}
	point.msnf = msnf

	sugars := sugarMasses{
		sucrose:      sampleInterval(comps.Sucrose, rng),
		glucose:      sampleInterval(comps.Glucose, rng),
		fructose:     sampleInterval(comps.Fructose, rng),
		maltodextrin: sampleInterval(comps.Maltodextrin, rng),
		polyols:      sampleInterval(comps.Polyols, rng),
	}
	point.addedSugar = sugars.sucrose + sugars.glucose + sugars.fructose + sugars.maltodextrin + sugars.polyols
	point.totalSugar = point.addedSugar + point.lactose

	addedPOD := addedPODFromMasses(sugars)
	addedPAC := addedPACFromMasses(sugars, profile.Functionals)
	lactosePOD := point.lactose * LactosePOD
	lactosePAC := lactosePACFromMass(point.lactose, profile.Functionals)
	point.pod = addedPOD + lactosePOD
	point.pac = addedPAC + lactosePAC

	if point.water <= 0 {
		total := point.fat + point.msnf + point.addedSugar + point.other
		point.water = math.Max(0, 1-total)
	}

	return point
}

// weightsToSolutionWithCoeffs converts weights using specific coefficients.
func (s *Solver) weightsToSolutionWithCoeffs(weights []float64, ids []IngredientID, names []string, lpp *lpProblem) *Solution {
	sol := &Solution{
		Weights: make(map[IngredientID]float64),
		Names:   make(map[IngredientID]string, len(ids)),
	}

	for i, w := range weights {
		id := ids[i]
		sol.Weights[id] = w
		sol.Names[id] = names[i]
	}

	// Compute achieved composition using the LP's coefficients
	var fat, msnf, sugar, other float64
	for i, w := range weights {
		fat += w * lpp.fatLo[i]
		msnf += w * lpp.msnfLo[i]
		sugar += w * lpp.sugarLo[i]
		other += w * lpp.otherLo[i]
	}

	sol.Achieved = PointComposition(fat, msnf, sugar, other)
	return sol
}

// ExtremePoints finds solutions at the extreme values of each ingredient.
// This explores the "corners" of the solution space by maximizing and
// minimizing each ingredient's weight.
func (s *Solver) ExtremePoints() ([]*Solution, error) {
	ids := s.Problem.IngredientIDs()
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
			solutions = append(solutions, s.weightsToSolution(weights, ids, names))
		}

		// Maximize w_i
		maxObj := make([]float64, n)
		maxObj[i] = -1
		_, weights, err = lpp.solve(maxObj)
		if err == nil {
			solutions = append(solutions, s.weightsToSolution(weights, ids, names))
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

	for id, wa := range a.Weights {
		wb, ok := b.Weights[id]
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
