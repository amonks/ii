package linear

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/convex/lp"
)

const tolerance = 1e-9

// Solver solves ice cream formulation problems.
type Solver struct {
	Problem *Problem
}

// NewSolver creates a solver for the given problem.
func NewSolver(p *Problem) (*Solver, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &Solver{Problem: p}, nil
}

// lpProblem represents the linear programming formulation.
// Variables: w_0, w_1, ..., w_{n-1} (ingredient weights)
//
// Constraints:
//   - sum(w_i) = 1                                    (mass balance)
//   - sum(w_i * fat_i) in [target.Fat.Lo, target.Fat.Hi]
//   - sum(w_i * msnf_i) in [target.MSNF.Lo, target.MSNF.Hi]
//   - sum(w_i * sugar_i) in [target.Sugar.Lo, target.Sugar.Hi]
//   - sum(w_i * other_i) in [target.Other.Lo, target.Other.Hi]
//   - sum(w_i * pod_i) in [targetPOD.Lo, targetPOD.Hi] (if set)
//   - sum(w_i * pac_i) in [targetPAC.Lo, targetPAC.Hi] (if set)
//   - w_i >= bound.Lo, w_i <= bound.Hi for each ingredient
//   - w_0 >= w_1 >= ... >= w_{n-1} if OrderConstraints
type lpProblem struct {
	n int // number of ingredients

	// Coefficient matrices (using midpoints of intervals for basic solving)
	fat   []float64
	msnf  []float64
	sugar []float64
	other []float64
	pod   []float64 // POD contribution per unit weight
	pac   []float64 // PAC contribution per unit weight

	// Bounds on each variable
	lower []float64
	upper []float64

	// Target intervals
	target    Composition
	targetPOD Interval
	targetPAC Interval

	// Order constraints
	orderConstraints bool
}

// buildLP creates the LP formulation using midpoints of ingredient composition intervals.
func (s *Solver) buildLP() *lpProblem {
	p := s.Problem
	n := len(p.Ingredients)

	lpp := &lpProblem{
		n:                n,
		fat:              make([]float64, n),
		msnf:             make([]float64, n),
		sugar:            make([]float64, n),
		other:            make([]float64, n),
		pod:              make([]float64, n),
		pac:              make([]float64, n),
		lower:            make([]float64, n),
		upper:            make([]float64, n),
		target:           p.Target,
		targetPOD:        p.TargetPOD,
		targetPAC:        p.TargetPAC,
		orderConstraints: p.OrderConstraints,
	}

	for i, ing := range p.Ingredients {
		// Use midpoints for coefficient estimation
		lpp.fat[i] = ing.Comp.Fat.Mid()
		lpp.msnf[i] = ing.Comp.MSNF.Mid()
		lpp.sugar[i] = ing.Comp.Sugar.Mid()
		lpp.other[i] = ing.Comp.Other.Mid()

		// POD/PAC coefficients: contribution per unit weight of ingredient
		// = (sugar content × sweetener POD/PAC) + (MSNF × lactose fraction × lactose POD/PAC)
		sugarContent := ing.Comp.Sugar.Mid()
		msnfContent := ing.Comp.MSNF.Mid()
		lactoseContent := msnfContent * LactoseFractionOfMSNF

		lpp.pod[i] = sugarContent*ing.Sweetener.POD + lactoseContent*LactosePOD
		lpp.pac[i] = sugarContent*ing.Sweetener.PAC + lactoseContent*LactosePAC

		// Weight bounds
		if bound, ok := p.WeightBounds[ing.Name]; ok {
			lpp.lower[i] = bound.Lo
			lpp.upper[i] = bound.Hi
		} else {
			lpp.lower[i] = 0
			lpp.upper[i] = 1
		}
	}

	return lpp
}

// buildLPWithCoeffs creates an LP with specific coefficient values.
func (s *Solver) buildLPWithCoeffs(fatCoeffs, msnfCoeffs, sugarCoeffs, otherCoeffs []float64) *lpProblem {
	p := s.Problem
	n := len(p.Ingredients)

	lpp := &lpProblem{
		n:                n,
		fat:              fatCoeffs,
		msnf:             msnfCoeffs,
		sugar:            sugarCoeffs,
		other:            otherCoeffs,
		pod:              make([]float64, n),
		pac:              make([]float64, n),
		lower:            make([]float64, n),
		upper:            make([]float64, n),
		target:           p.Target,
		targetPOD:        p.TargetPOD,
		targetPAC:        p.TargetPAC,
		orderConstraints: p.OrderConstraints,
	}

	for i, ing := range p.Ingredients {
		if bound, ok := p.WeightBounds[ing.Name]; ok {
			lpp.lower[i] = bound.Lo
			lpp.upper[i] = bound.Hi
		} else {
			lpp.lower[i] = 0
			lpp.upper[i] = 1
		}

		// POD/PAC coefficients (using the provided sugar/msnf coefficients)
		lactoseContent := msnfCoeffs[i] * LactoseFractionOfMSNF
		lpp.pod[i] = sugarCoeffs[i]*ing.Sweetener.POD + lactoseContent*LactosePOD
		lpp.pac[i] = sugarCoeffs[i]*ing.Sweetener.PAC + lactoseContent*LactosePAC
	}

	return lpp
}

// solve runs the LP with a given objective, returns optimal value and solution.
// objective: coefficients for minimization
// Returns (objective value, solution weights, error)
func (lpp *lpProblem) solve(objective []float64) (float64, []float64, error) {
	n := lpp.n

	// Count inequality constraints:
	// - 8 inequalities: 2 per composition component (lo <= sum <= hi)
	// - 0-2 for POD target (if set)
	// - 0-2 for PAC target (if set)
	// - 2n inequalities: lower and upper bounds on each variable
	// - n-1 inequalities: ordering constraints (if enabled)

	numIneq := 8 + 2*n
	hasPOD := lpp.targetPOD.Hi > 0
	hasPAC := lpp.targetPAC.Hi > 0
	if hasPOD {
		numIneq += 2
	}
	if hasPAC {
		numIneq += 2
	}
	if lpp.orderConstraints {
		numIneq += n - 1
	}

	// Build inequality matrix G and vector h: Gx <= h
	G := mat.NewDense(numIneq, n, nil)
	h := make([]float64, numIneq)

	row := 0

	// Fat constraints: target.Lo <= sum(w_i * fat_i) <= target.Hi
	// Rewrite as: -sum <= -Lo and sum <= Hi
	for i := 0; i < n; i++ {
		G.Set(row, i, -lpp.fat[i])
	}
	h[row] = -lpp.target.Fat.Lo
	row++

	for i := 0; i < n; i++ {
		G.Set(row, i, lpp.fat[i])
	}
	h[row] = lpp.target.Fat.Hi
	row++

	// MSNF constraints
	for i := 0; i < n; i++ {
		G.Set(row, i, -lpp.msnf[i])
	}
	h[row] = -lpp.target.MSNF.Lo
	row++

	for i := 0; i < n; i++ {
		G.Set(row, i, lpp.msnf[i])
	}
	h[row] = lpp.target.MSNF.Hi
	row++

	// Sugar constraints
	for i := 0; i < n; i++ {
		G.Set(row, i, -lpp.sugar[i])
	}
	h[row] = -lpp.target.Sugar.Lo
	row++

	for i := 0; i < n; i++ {
		G.Set(row, i, lpp.sugar[i])
	}
	h[row] = lpp.target.Sugar.Hi
	row++

	// Other constraints
	for i := 0; i < n; i++ {
		G.Set(row, i, -lpp.other[i])
	}
	h[row] = -lpp.target.Other.Lo
	row++

	for i := 0; i < n; i++ {
		G.Set(row, i, lpp.other[i])
	}
	h[row] = lpp.target.Other.Hi
	row++

	// POD constraints (if set)
	if hasPOD {
		for i := 0; i < n; i++ {
			G.Set(row, i, -lpp.pod[i])
		}
		h[row] = -lpp.targetPOD.Lo
		row++

		for i := 0; i < n; i++ {
			G.Set(row, i, lpp.pod[i])
		}
		h[row] = lpp.targetPOD.Hi
		row++
	}

	// PAC constraints (if set)
	if hasPAC {
		for i := 0; i < n; i++ {
			G.Set(row, i, -lpp.pac[i])
		}
		h[row] = -lpp.targetPAC.Lo
		row++

		for i := 0; i < n; i++ {
			G.Set(row, i, lpp.pac[i])
		}
		h[row] = lpp.targetPAC.Hi
		row++
	}

	// Variable bounds: lower_i <= w_i <= upper_i
	// Rewrite as: -w_i <= -lower_i and w_i <= upper_i
	for i := 0; i < n; i++ {
		G.Set(row, i, -1)
		h[row] = -lpp.lower[i]
		row++
	}

	for i := 0; i < n; i++ {
		G.Set(row, i, 1)
		h[row] = lpp.upper[i]
		row++
	}

	// Ordering constraints: w_i >= w_{i+1} => w_{i+1} - w_i <= 0
	if lpp.orderConstraints {
		for i := 0; i < n-1; i++ {
			G.Set(row, i, -1)
			G.Set(row, i+1, 1)
			h[row] = 0
			row++
		}
	}

	// Equality constraint: sum(w_i) = 1
	A := mat.NewDense(1, n, nil)
	for i := 0; i < n; i++ {
		A.Set(0, i, 1)
	}
	b := []float64{1.0}

	// Convert general LP to standard form
	// General form: min c'x, s.t. Gx <= h, Ax = b
	// Standard form: min c'x, s.t. Ax = b, x >= 0
	cNew, aNew, bNew := lp.Convert(objective, G, h, A, b)

	// Solve in standard form
	opt, xNew, err := lp.Simplex(cNew, aNew, bNew, tolerance, nil)
	if err != nil {
		return 0, nil, err
	}

	// Extract original variables (first n elements, before slack variables)
	x := make([]float64, n)
	copy(x, xNew[:n])

	return opt, x, nil
}

// Feasible checks if the problem has any feasible solution.
func (s *Solver) Feasible() (bool, error) {
	lpp := s.buildLP()

	// Use zero objective (any feasible point)
	objective := make([]float64, lpp.n)

	_, _, err := lpp.solve(objective)
	if err != nil {
		// Check if error is infeasibility vs numerical issues
		return false, nil
	}
	return true, nil
}

// FindBounds computes the min/max feasible weight for each ingredient.
func (s *Solver) FindBounds() (*Bounds, error) {
	lpp := s.buildLP()
	n := lpp.n
	names := s.Problem.IngredientNames()

	bounds := &Bounds{
		WeightRanges: make(map[string]Interval),
		Feasible:     false,
	}

	// First check feasibility
	objective := make([]float64, n)
	_, _, err := lpp.solve(objective)
	if err != nil {
		return bounds, nil // infeasible
	}
	bounds.Feasible = true

	// For each ingredient, find min and max
	for i := 0; i < n; i++ {
		// Minimize w_i
		minObj := make([]float64, n)
		minObj[i] = 1
		minVal, _, err := lpp.solve(minObj)
		if err != nil {
			return nil, fmt.Errorf("error finding min for %s: %w", names[i], err)
		}

		// Maximize w_i (minimize -w_i)
		maxObj := make([]float64, n)
		maxObj[i] = -1
		maxVal, _, err := lpp.solve(maxObj)
		if err != nil {
			return nil, fmt.Errorf("error finding max for %s: %w", names[i], err)
		}

		bounds.WeightRanges[names[i]] = Interval{
			Lo: math.Max(0, minVal),
			Hi: math.Min(1, -maxVal),
		}
	}

	return bounds, nil
}

// FindSolution finds a single feasible solution (if one exists).
func (s *Solver) FindSolution() (*Solution, error) {
	lpp := s.buildLP()
	names := s.Problem.IngredientNames()

	// Use zero objective to find any feasible point
	objective := make([]float64, lpp.n)
	_, x, err := lpp.solve(objective)
	if err != nil {
		return nil, fmt.Errorf("no feasible solution: %w", err)
	}

	return s.weightsToSolution(x, names), nil
}

// weightsToSolution converts raw weights to a Solution.
func (s *Solver) weightsToSolution(weights []float64, names []string) *Solution {
	sol := &Solution{
		Weights: make(map[string]float64),
	}

	for i, w := range weights {
		sol.Weights[names[i]] = w
	}

	// Compute achieved composition using ingredient midpoints
	var fat, msnf, sugar, other float64
	for i, ing := range s.Problem.Ingredients {
		w := weights[i]
		fat += w * ing.Comp.Fat.Mid()
		msnf += w * ing.Comp.MSNF.Mid()
		sugar += w * ing.Comp.Sugar.Mid()
		other += w * ing.Comp.Other.Mid()
	}

	sol.Achieved = PointComposition(fat, msnf, sugar, other)
	return sol
}
