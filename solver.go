package creamery

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/convex/lp"
)

const tolerance = 1e-9

type coefficientSet struct {
	fat        []float64
	msnf       []float64
	sugar      []float64
	other      []float64
	protein    []float64
	lactose    []float64
	totalSugar []float64
	water      []float64
	pod        []float64
	pac        []float64
}

func newCoefficientSet(n int) coefficientSet {
	return coefficientSet{
		fat:        make([]float64, n),
		msnf:       make([]float64, n),
		sugar:      make([]float64, n),
		other:      make([]float64, n),
		protein:    make([]float64, n),
		lactose:    make([]float64, n),
		totalSugar: make([]float64, n),
		water:      make([]float64, n),
		pod:        make([]float64, n),
		pac:        make([]float64, n),
	}
}

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

	// Coefficient intervals for each constituent
	fatLo        []float64
	fatHi        []float64
	msnfLo       []float64
	msnfHi       []float64
	sugarLo      []float64 // added sugars (non-lactose)
	sugarHi      []float64
	otherLo      []float64
	otherHi      []float64
	proteinLo    []float64
	proteinHi    []float64
	lactoseLo    []float64
	lactoseHi    []float64
	totalSugarLo []float64
	totalSugarHi []float64
	waterLo      []float64
	waterHi      []float64
	podLo        []float64
	podHi        []float64
	pacLo        []float64
	pacHi        []float64

	// Bounds on each variable
	lower []float64
	upper []float64

	// Target intervals
	target    FormulationTarget
	targetPOD Interval
	targetPAC Interval

	// Order constraints
	orderConstraints bool

	// Optional linear constraints
	constraints []LinearConstraint

	ids       []IngredientID
	names     []string
	idToIndex map[IngredientID]int
}

// buildLP creates the LP formulation using midpoints of ingredient composition intervals.
func (s *Solver) buildLP() *lpProblem {
	p := s.Problem
	n := len(p.Specs)
	ids := p.IngredientIDs()
	names := p.IngredientNames()
	idIndex := make(map[IngredientID]int, n)
	for i, id := range ids {
		idIndex[id] = i
	}

	lpp := &lpProblem{
		n:                n,
		fatLo:            make([]float64, n),
		fatHi:            make([]float64, n),
		msnfLo:           make([]float64, n),
		msnfHi:           make([]float64, n),
		sugarLo:          make([]float64, n),
		sugarHi:          make([]float64, n),
		otherLo:          make([]float64, n),
		otherHi:          make([]float64, n),
		proteinLo:        make([]float64, n),
		proteinHi:        make([]float64, n),
		lactoseLo:        make([]float64, n),
		lactoseHi:        make([]float64, n),
		totalSugarLo:     make([]float64, n),
		totalSugarHi:     make([]float64, n),
		waterLo:          make([]float64, n),
		waterHi:          make([]float64, n),
		podLo:            make([]float64, n),
		podHi:            make([]float64, n),
		pacLo:            make([]float64, n),
		pacHi:            make([]float64, n),
		lower:            make([]float64, n),
		upper:            make([]float64, n),
		target:           p.Target,
		targetPOD:        p.Target.POD,
		targetPAC:        p.Target.PAC,
		orderConstraints: p.OrderConstraints,
		constraints:      p.Constraints,
		ids:              ids,
		names:            names,
		idToIndex:        idIndex,
	}

	for i, spec := range p.Specs {
		profile := p.profileForIndex(i)
		components := profile.Components
		fat := components.Fat
		msnf := profile.MSNFInterval()
		sugar := profile.AddedSugarsInterval()
		other := profile.OtherSolidsInterval()
		protein := profile.ProteinInterval()
		lactose := profile.LactoseInterval()
		totalSugar := profile.TotalSugarInterval()
		water := profile.WaterInterval()

		lpp.fatLo[i] = fat.Lo
		lpp.fatHi[i] = fat.Hi
		lpp.msnfLo[i] = msnf.Lo
		lpp.msnfHi[i] = msnf.Hi
		lpp.sugarLo[i] = sugar.Lo
		lpp.sugarHi[i] = sugar.Hi
		lpp.otherLo[i] = other.Lo
		lpp.otherHi[i] = other.Hi
		lpp.proteinLo[i] = protein.Lo
		lpp.proteinHi[i] = protein.Hi
		lpp.lactoseLo[i] = lactose.Lo
		lpp.lactoseHi[i] = lactose.Hi
		lpp.totalSugarLo[i] = totalSugar.Lo
		lpp.totalSugarHi[i] = totalSugar.Hi
		lpp.waterLo[i] = water.Lo
		lpp.waterHi[i] = water.Hi

		pod := profile.PODInterval()
		pac := profile.PACInterval()
		lpp.podLo[i] = pod.Lo
		lpp.podHi[i] = pod.Hi
		lpp.pacLo[i] = pac.Lo
		lpp.pacHi[i] = pac.Hi

		// Weight bounds
		if bound, ok := p.WeightBounds[spec.ID]; ok {
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
func (s *Solver) buildLPWithCoeffs(coeffs coefficientSet) *lpProblem {
	p := s.Problem
	n := len(p.Specs)
	ids := p.IngredientIDs()
	names := p.IngredientNames()
	idIndex := make(map[IngredientID]int, n)
	for i, id := range ids {
		idIndex[id] = i
	}

	lpp := &lpProblem{
		n:                n,
		fatLo:            make([]float64, n),
		fatHi:            make([]float64, n),
		msnfLo:           make([]float64, n),
		msnfHi:           make([]float64, n),
		sugarLo:          make([]float64, n),
		sugarHi:          make([]float64, n),
		otherLo:          make([]float64, n),
		otherHi:          make([]float64, n),
		proteinLo:        make([]float64, n),
		proteinHi:        make([]float64, n),
		lactoseLo:        make([]float64, n),
		lactoseHi:        make([]float64, n),
		totalSugarLo:     make([]float64, n),
		totalSugarHi:     make([]float64, n),
		waterLo:          make([]float64, n),
		waterHi:          make([]float64, n),
		podLo:            make([]float64, n),
		podHi:            make([]float64, n),
		pacLo:            make([]float64, n),
		pacHi:            make([]float64, n),
		lower:            make([]float64, n),
		upper:            make([]float64, n),
		target:           p.Target,
		targetPOD:        p.Target.POD,
		targetPAC:        p.Target.PAC,
		orderConstraints: p.OrderConstraints,
		constraints:      p.Constraints,
		ids:              ids,
		names:            names,
		idToIndex:        idIndex,
	}

	for i := range p.Specs {
		spec := p.Specs[i]
		fat := coeffs.fat[i]
		msnf := coeffs.msnf[i]
		sugar := coeffs.sugar[i]
		other := coeffs.other[i]
		protein := coeffs.protein[i]
		lactose := coeffs.lactose[i]
		totalSugar := coeffs.totalSugar[i]
		water := coeffs.water[i]
		pod := coeffs.pod[i]
		pac := coeffs.pac[i]

		lpp.fatLo[i] = fat
		lpp.fatHi[i] = fat
		lpp.msnfLo[i] = msnf
		lpp.msnfHi[i] = msnf
		lpp.sugarLo[i] = sugar
		lpp.sugarHi[i] = sugar
		lpp.otherLo[i] = other
		lpp.otherHi[i] = other
		lpp.proteinLo[i] = protein
		lpp.proteinHi[i] = protein
		lpp.lactoseLo[i] = lactose
		lpp.lactoseHi[i] = lactose
		lpp.totalSugarLo[i] = totalSugar
		lpp.totalSugarHi[i] = totalSugar
		lpp.waterLo[i] = water
		lpp.waterHi[i] = water
		lpp.podLo[i] = pod
		lpp.podHi[i] = pod
		lpp.pacLo[i] = pac
		lpp.pacHi[i] = pac

		if bound, ok := p.WeightBounds[spec.ID]; ok {
			lpp.lower[i] = bound.Lo
			lpp.upper[i] = bound.Hi
		} else {
			lpp.lower[i] = 0
			lpp.upper[i] = 1
		}

	}

	return lpp
}

// solve runs the LP with a given objective, returns optimal value and solution.
// objective: coefficients for minimization
// Returns (objective value, solution weights, error)
func (lpp *lpProblem) solve(objective []float64) (float64, []float64, error) {
	n := lpp.n

	targetComp := lpp.target.Composition
	type componentConstraint struct {
		lo     []float64
		hi     []float64
		target Interval
	}
	componentConstraints := []componentConstraint{
		{lpp.fatLo, lpp.fatHi, targetComp.Fat},
		{lpp.msnfLo, lpp.msnfHi, targetComp.MSNF},
		{lpp.sugarLo, lpp.sugarHi, targetComp.Sugar},
		{lpp.otherLo, lpp.otherHi, targetComp.Other},
	}

	if proteinTarget := lpp.target.ProteinInterval(); intervalSpecified(proteinTarget) {
		componentConstraints = append(componentConstraints, componentConstraint{lpp.proteinLo, lpp.proteinHi, proteinTarget})
	}
	if lactoseTarget := lpp.target.LactoseInterval(); intervalSpecified(lactoseTarget) {
		componentConstraints = append(componentConstraints, componentConstraint{lpp.lactoseLo, lpp.lactoseHi, lactoseTarget})
	}
	if addedTarget := lpp.target.AddedSugarsInterval(); intervalSpecified(addedTarget) {
		componentConstraints = append(componentConstraints, componentConstraint{lpp.sugarLo, lpp.sugarHi, addedTarget})
	}
	if totalTarget := lpp.target.TotalSugarsInterval(); intervalSpecified(totalTarget) {
		componentConstraints = append(componentConstraints, componentConstraint{lpp.totalSugarLo, lpp.totalSugarHi, totalTarget})
	}
	if waterTarget := lpp.target.WaterInterval(); intervalSpecified(waterTarget) {
		componentConstraints = append(componentConstraints, componentConstraint{lpp.waterLo, lpp.waterHi, waterTarget})
	}

	// Count inequality constraints:
	componentRows := len(componentConstraints) * 2
	numIneq := componentRows + 2*n
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
	for _, constraint := range lpp.constraints {
		if constraint.Upper < math.Inf(1) {
			numIneq++
		}
		if constraint.Lower > math.Inf(-1) {
			numIneq++
		}
	}

	// Build inequality matrix G and vector h: Gx <= h
	G := mat.NewDense(numIneq, n, nil)
	h := make([]float64, numIneq)

	row := 0

	for _, comp := range componentConstraints {
		for i := 0; i < n; i++ {
			G.Set(row, i, -comp.lo[i])
		}
		h[row] = -comp.target.Lo
		row++

		for i := 0; i < n; i++ {
			G.Set(row, i, comp.hi[i])
		}
		h[row] = comp.target.Hi
		row++
	}

	// POD constraints (if set)
	if hasPOD {
		for i := 0; i < n; i++ {
			G.Set(row, i, -lpp.podLo[i])
		}
		h[row] = -lpp.targetPOD.Lo
		row++

		for i := 0; i < n; i++ {
			G.Set(row, i, lpp.podHi[i])
		}
		h[row] = lpp.targetPOD.Hi
		row++
	}

	// PAC constraints (if set)
	if hasPAC {
		for i := 0; i < n; i++ {
			G.Set(row, i, -lpp.pacLo[i])
		}
		h[row] = -lpp.targetPAC.Lo
		row++

		for i := 0; i < n; i++ {
			G.Set(row, i, lpp.pacHi[i])
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

	// Additional linear constraints
	for _, constraint := range lpp.constraints {
		if constraint.Upper < math.Inf(1) {
			for id, coeff := range constraint.Coeffs {
				if idx, ok := lpp.idToIndex[id]; ok {
					G.Set(row, idx, coeff)
				}
			}
			h[row] = constraint.Upper
			row++
		}
		if constraint.Lower > math.Inf(-1) {
			for id, coeff := range constraint.Coeffs {
				if idx, ok := lpp.idToIndex[id]; ok {
					G.Set(row, idx, -coeff)
				}
			}
			h[row] = -constraint.Lower
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

func intervalSpecified(iv Interval) bool {
	return iv.Lo != 0 || iv.Hi != 0
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
	ids := s.Problem.IngredientIDs()
	names := s.Problem.IngredientNames()

	bounds := &Bounds{
		WeightRanges: make(map[IngredientID]Interval),
		Names:        make(map[IngredientID]string, len(ids)),
		Feasible:     false,
	}
	for i, id := range ids {
		bounds.Names[id] = names[i]
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

		bounds.WeightRanges[ids[i]] = Interval{
			Lo: math.Max(0, minVal),
			Hi: math.Min(1, -maxVal),
		}
	}

	return bounds, nil
}

// FindSolution finds a single feasible solution (if one exists).
func (s *Solver) FindSolution() (*Solution, error) {
	lpp := s.buildLP()
	ids := s.Problem.IngredientIDs()
	names := s.Problem.IngredientNames()

	// Use zero objective to find any feasible point
	objective := make([]float64, lpp.n)
	_, x, err := lpp.solve(objective)
	if err != nil {
		return nil, fmt.Errorf("no feasible solution: %w", err)
	}

	return s.weightsToSolution(x, ids, names), nil
}

// weightsToSolution converts raw weights to a Solution.
func (s *Solver) weightsToSolution(weights []float64, ids []IngredientID, names []string) *Solution {
	sol := &Solution{
		Weights: make(map[IngredientID]float64),
		Names:   make(map[IngredientID]string, len(ids)),
		Lots:    make(map[IngredientID]IngredientLot, len(ids)),
	}

	for i, w := range weights {
		id := ids[i]
		sol.Weights[id] = w
		sol.Names[id] = names[i]
		if lot, ok := s.Problem.LotByID(id); ok {
			sol.Lots[id] = lot
		}
	}

	components := sumComponents(weights, s.Problem.profiles)
	sol.Components = components
	sol.Achieved = Composition{
		Fat:   components.Fat,
		MSNF:  components.MSNF,
		Sugar: components.AddedSugarsInterval(),
		Other: components.OtherSolids,
	}
	return sol
}

func sumComponents(weights []float64, profiles []ConstituentProfile) ConstituentComponents {
	var agg ConstituentComponents
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		comps := profiles[i].Components
		agg.Water = agg.Water.Add(comps.Water.Scale(w))
		agg.Fat = agg.Fat.Add(comps.Fat.Scale(w))
		agg.MSNF = agg.MSNF.Add(profiles[i].MSNFInterval().Scale(w))
		agg.Protein = agg.Protein.Add(comps.Protein.Scale(w))
		agg.Lactose = agg.Lactose.Add(comps.Lactose.Scale(w))
		agg.Sucrose = agg.Sucrose.Add(comps.Sucrose.Scale(w))
		agg.Glucose = agg.Glucose.Add(comps.Glucose.Scale(w))
		agg.Fructose = agg.Fructose.Add(comps.Fructose.Scale(w))
		agg.Maltodextrin = agg.Maltodextrin.Add(comps.Maltodextrin.Scale(w))
		agg.Polyols = agg.Polyols.Add(comps.Polyols.Scale(w))
		agg.Ash = agg.Ash.Add(comps.Ash.Scale(w))
		agg.OtherSolids = agg.OtherSolids.Add(comps.OtherSolids.Scale(w))
	}
	return agg
}
