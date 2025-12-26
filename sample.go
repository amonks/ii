package creamery

import (
	"errors"
	"math"
	"math/rand"
	"sort"

	"github.com/go-nlopt/nlopt"
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
			for j := range s.Problem.slots {
				profile := s.Problem.profileForIndex(j)
				point := sampleProfilePoint(profile, rng)
				coeffs.set(componentFat, j, point.fat)
				coeffs.set(componentMSNF, j, point.msnf)
				coeffs.set(componentProtein, j, point.protein)
				coeffs.set(componentLactose, j, point.lactose)
				coeffs.set(componentOther, j, point.other)
				coeffs.set(componentWater, j, point.water)
				coeffs.set(componentSucrose, j, point.sucrose)
				coeffs.set(componentGlucose, j, point.glucose)
				coeffs.set(componentFructose, j, point.fructose)
				coeffs.set(componentMaltodextrin, j, point.maltodextrin)
				coeffs.set(componentPolyols, j, point.polyols)
				coeffs.set(componentAsh, j, point.ash)
				coeffs.set(componentAdded, j, point.addedSugar)
				coeffs.set(componentTotal, j, point.totalSugar)
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

		_, weights, err := s.solve(lpp, objective)
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
	fat          float64
	msnf         float64
	sucrose      float64
	glucose      float64
	fructose     float64
	maltodextrin float64
	polyols      float64
	addedSugar   float64
	other        float64
	protein      float64
	lactose      float64
	totalSugar   float64
	water        float64
	ash          float64
	pod          float64
	pac          float64
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
	point.ash = ash
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
	point.sucrose = sugars.sucrose
	point.glucose = sugars.glucose
	point.fructose = sugars.fructose
	point.maltodextrin = sugars.maltodextrin
	point.polyols = sugars.polyols
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
		Lots:    make(map[IngredientID]Lot, len(ids)),
	}

	blend := make([]Portion, 0, len(weights))
	for i, w := range weights {
		id := ids[i]
		sol.Weights[id] = w
		sol.Names[id] = names[i]
		if lot, ok := s.Problem.LotByID(id); ok {
			sol.Lots[id] = lot
			if w > 0 {
				blend = append(blend, Portion{
					Lot:      lot,
					Fraction: w,
				})
			}
		}
	}
	sol.Blend = Blend{Components: blend}

	components := sumComponents(weights, s.Problem.slots)
	sol.Components = components

	achieved := ComponentFractions{}
	for key, pair := range lpp.componentValues {
		value := 0.0
		for i, w := range weights {
			value += w * pair.lo[i]
		}
		applyComponentValue(&achieved, key, value)
	}
	sol.Achieved = EnsureWater(achieved)
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
		_, weights, err := s.solve(lpp, minObj)
		if err == nil {
			solutions = append(solutions, s.weightsToSolution(weights, ids, names))
		}

		// Maximize w_i
		maxObj := make([]float64, n)
		maxObj[i] = -1
		_, weights, err = s.solve(lpp, maxObj)
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

type solutionPreferenceScore struct {
	solution  *Solution
	score     float64
	viscosity float64
}

func reorderSolutionsByPreference(solutions []*Solution, pref RecipePreference, opts MixOptions) []*Solution {
	if len(solutions) == 0 {
		return solutions
	}
	pref = normalizeRecipePreference(pref)

	scored := make([]solutionPreferenceScore, 0, len(solutions))
	fallback := make([]*Solution, 0)

	for _, sol := range solutions {
		score, err := sol.Score(pref, opts)
		if err != nil {
			fallback = append(fallback, sol)
			continue
		}
		visc := 0.0
		if _, process, snapErr := sol.Snapshot(opts); snapErr == nil {
			visc = process.ViscosityAtServe
		}
		scored = append(scored, solutionPreferenceScore{
			solution:  sol,
			score:     score,
			viscosity: visc,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].viscosity < scored[j].viscosity
		}
		return scored[i].score > scored[j].score
	})

	reordered := make([]*Solution, 0, len(solutions))
	for _, entry := range scored {
		reordered = append(reordered, entry.solution)
	}
	reordered = append(reordered, fallback...)
	return reordered
}

// CompareSolutions returns 1 if a scores higher than b, -1 if lower, and 0 if
// the scores are effectively equal under the supplied preference curves.
func CompareSolutions(a, b *Solution, pref RecipePreference, opts MixOptions) (int, error) {
	if a == nil || b == nil {
		return 0, errors.New("solutions must be non-nil")
	}
	scoreA, err := a.Score(pref, opts)
	if err != nil {
		return 0, err
	}
	scoreB, err := b.Score(pref, opts)
	if err != nil {
		return 0, err
	}
	diff := scoreA - scoreB
	const epsilon = 1e-9
	if diff > epsilon {
		return 1, nil
	}
	if diff < -epsilon {
		return -1, nil
	}
	return 0, nil
}

// DiverseSamples generates a set of diverse feasible solutions by
// combining extreme points with random samples.
func (s *Solver) DiverseSamples(count int, rng *rand.Rand) ([]*Solution, error) {
	if count <= 0 {
		count = 1
	}
	// Start with extreme points
	extremes, err := s.ExtremePoints()
	if err != nil {
		return nil, err
	}

	// Always include at least some randomized samples so we can rank the
	// candidates by texture (viscosity) instead of returning an arbitrary
	// extreme point.
	randomCount := count - len(extremes)
	if randomCount < count {
		randomCount = count
	}
	if randomCount <= 0 {
		randomCount = count
	}
	randoms, err := s.Sample(randomCount*2, true, rng) // oversample, will dedupe
	if err != nil {
		return nil, err
	}

	all := append(extremes, randoms...)
	unique := deduplicateSolutions(all, 0.01)
	if len(unique) == 0 {
		return unique, nil
	}

	unique = reorderSolutionsByPreference(unique, DefaultRecipePreference(), MixOptions{})

	if len(unique) > count {
		return unique[:count], nil
	}
	return unique, nil
}

// Optimize returns the highest-scoring recipe according to the default
// preference curves (viscosity, sweetness, ice fraction). Sampling fallback
// has been removed so failures are surfaced to the caller.
func (s *Solver) Optimize(rng *rand.Rand) (*Solution, error) {
	_ = rng
	return s.OptimizeWithPreference(DefaultRecipePreference())
}

func (s *Solver) OptimizeWithPreference(pref RecipePreference) (*Solution, error) {
	lpp := s.buildLP()
	ids := s.Problem.IngredientIDs()
	names := s.Problem.IngredientNames()
	pref = normalizeRecipePreference(pref)

	mixOpts := MixOptions{}
	var snapshotErr error

	opts := s.opts
	opts.NLoptAlgorithm = nlopt.LN_COBYLA

	_, weights, err := lpp.solveWithObjective(opts, func(x, grad []float64) float64 {
		if grad != nil {
			for i := range grad {
				grad[i] = 0
			}
		}
		snapshot, process, snapErr := s.snapshotFromWeights(x, mixOpts)
		if snapErr != nil {
			if snapshotErr == nil {
				snapshotErr = snapErr
			}
			return 1
		}
		score := pref.Score(snapshot, process)
		return 1 - score
	})
	if err != nil {
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		return nil, err
	}
	if len(weights) == 0 {
		return nil, errors.New("optimizer returned no weights")
	}
	return s.weightsToSolution(weights, ids, names), nil
}
