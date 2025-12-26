package creamery

import (
	"errors"
	"fmt"
	"math"

	"github.com/go-nlopt/nlopt"
)

// SolverOptions configure solver behavior for the NLopt backend.
type SolverOptions struct {
	NLoptAlgorithm      int
	ConstraintTolerance float64
	MaxEval             int
}

var (
	defaultNLoptAlgorithm = nlopt.LD_SLSQP
	defaultConstraintTol  = 1e-7
	defaultMaxEval        = 2000
)

func applySolverOptionDefaults(opts SolverOptions) SolverOptions {
	if opts.ConstraintTolerance <= 0 {
		opts.ConstraintTolerance = defaultConstraintTol
	}
	if opts.MaxEval <= 0 {
		opts.MaxEval = defaultMaxEval
	}
	if opts.NLoptAlgorithm == 0 {
		opts.NLoptAlgorithm = defaultNLoptAlgorithm
	}
	return opts
}

type componentKey string

const (
	componentFat          componentKey = "fat"
	componentMSNF         componentKey = "msnf"
	componentProtein      componentKey = "protein"
	componentLactose      componentKey = "lactose"
	componentSucrose      componentKey = "sucrose"
	componentGlucose      componentKey = "glucose"
	componentFructose     componentKey = "fructose"
	componentMaltodextrin componentKey = "maltodextrin"
	componentPolyols      componentKey = "polyols"
	componentAsh          componentKey = "ash"
	componentOther        componentKey = "other"
	componentWater        componentKey = "water"
	componentAdded        componentKey = "added_sugar"
	componentTotal        componentKey = "total_sugar"
)

var componentKeyOrder = []componentKey{
	componentFat,
	componentMSNF,
	componentProtein,
	componentLactose,
	componentSucrose,
	componentGlucose,
	componentFructose,
	componentMaltodextrin,
	componentPolyols,
	componentAsh,
	componentOther,
	componentWater,
	componentAdded,
	componentTotal,
}

type coeffPair struct {
	lo []float64
	hi []float64
}

type coefficientSet struct {
	components map[componentKey][]float64
	pod        []float64
	pac        []float64
}

func newCoefficientSet(n int) coefficientSet {
	set := coefficientSet{
		components: make(map[componentKey][]float64, len(componentKeyOrder)),
		pod:        make([]float64, n),
		pac:        make([]float64, n),
	}
	for _, key := range componentKeyOrder {
		set.components[key] = make([]float64, n)
	}
	return set
}

func (c coefficientSet) set(key componentKey, index int, value float64) {
	if arr, ok := c.components[key]; ok {
		arr[index] = value
	}
}

func componentExtractor(key componentKey) func(ConstituentProfile) Interval {
	switch key {
	case componentFat:
		return func(profile ConstituentProfile) Interval { return profile.Components.Fat }
	case componentMSNF:
		return func(profile ConstituentProfile) Interval { return profile.MSNFInterval() }
	case componentProtein:
		return func(profile ConstituentProfile) Interval { return profile.ProteinInterval() }
	case componentLactose:
		return func(profile ConstituentProfile) Interval { return profile.LactoseInterval() }
	case componentSucrose:
		return func(profile ConstituentProfile) Interval { return profile.Components.Sucrose }
	case componentGlucose:
		return func(profile ConstituentProfile) Interval { return profile.Components.Glucose }
	case componentFructose:
		return func(profile ConstituentProfile) Interval { return profile.Components.Fructose }
	case componentMaltodextrin:
		return func(profile ConstituentProfile) Interval { return profile.Components.Maltodextrin }
	case componentPolyols:
		return func(profile ConstituentProfile) Interval { return profile.Components.Polyols }
	case componentAsh:
		return func(profile ConstituentProfile) Interval { return profile.Components.Ash }
	case componentOther:
		return func(profile ConstituentProfile) Interval { return profile.OtherSolidsInterval() }
	case componentWater:
		return func(profile ConstituentProfile) Interval { return profile.WaterInterval() }
	case componentAdded:
		return func(profile ConstituentProfile) Interval { return profile.AddedSugarsInterval() }
	case componentTotal:
		return func(profile ConstituentProfile) Interval { return profile.TotalSugarInterval() }
	default:
		return nil
	}
}

func targetIntervalForKey(target FormulationTarget, key componentKey) Interval {
	switch key {
	case componentFat:
		return target.FatInterval()
	case componentMSNF:
		return target.MSNFInterval()
	case componentProtein:
		return target.Components.Protein
	case componentLactose:
		return target.Components.Lactose
	case componentSucrose:
		return target.Components.Sucrose
	case componentGlucose:
		return target.Components.Glucose
	case componentFructose:
		return target.Components.Fructose
	case componentMaltodextrin:
		return target.Components.Maltodextrin
	case componentPolyols:
		return target.Components.Polyols
	case componentAsh:
		return target.Components.Ash
	case componentOther:
		return target.Components.OtherSolids
	case componentWater:
		return target.WaterInterval()
	case componentAdded:
		return target.AddedSugarsInterval()
	case componentTotal:
		return target.TotalSugarsInterval()
	default:
		return Interval{}
	}
}

func applyComponentValue(f *ComponentFractions, key componentKey, value float64) {
	if f == nil {
		return
	}
	switch key {
	case componentFat:
		f.Fat = Point(value)
	case componentMSNF:
		f.MSNF = Point(value)
	case componentProtein:
		f.Protein = Point(value)
	case componentLactose:
		f.Lactose = Point(value)
	case componentSucrose:
		f.Sucrose = Point(value)
	case componentGlucose:
		f.Glucose = Point(value)
	case componentFructose:
		f.Fructose = Point(value)
	case componentMaltodextrin:
		f.Maltodextrin = Point(value)
	case componentPolyols:
		f.Polyols = Point(value)
	case componentAsh:
		f.Ash = Point(value)
	case componentOther:
		f.OtherSolids = Point(value)
	case componentWater:
		f.Water = Point(value)
	}
}

func newComponentPairs(n int) map[componentKey]*coeffPair {
	pairs := make(map[componentKey]*coeffPair, len(componentKeyOrder))
	for _, key := range componentKeyOrder {
		pairs[key] = &coeffPair{
			lo: make([]float64, n),
			hi: make([]float64, n),
		}
	}
	return pairs
}

func componentPairsFromSlots(slots []ingredientSlot) map[componentKey]*coeffPair {
	pairs := newComponentPairs(len(slots))
	for i, slot := range slots {
		if slot.definition == nil {
			continue
		}
		profile := slot.lot.EffectiveProfile()
		for _, key := range componentKeyOrder {
			extractor := componentExtractor(key)
			if extractor == nil {
				continue
			}
			interval := extractor(profile)
			pair := pairs[key]
			pair.lo[i] = interval.Lo
			pair.hi[i] = interval.Hi
		}
	}
	return pairs
}

func componentPairsFromCoeffs(coeffs coefficientSet, n int) map[componentKey]*coeffPair {
	pairs := newComponentPairs(n)
	for key, values := range coeffs.components {
		pair, ok := pairs[key]
		if !ok {
			continue
		}
		copy(pair.lo, values)
		copy(pair.hi, values)
	}
	return pairs
}

func buildComponentConstraints(values map[componentKey]*coeffPair, target FormulationTarget) []lpComponentConstraint {
	constraints := make([]lpComponentConstraint, 0, len(values))
	for _, key := range componentKeyOrder {
		pair, ok := values[key]
		if !ok {
			continue
		}
		targetInterval := targetIntervalForKey(target, key)
		if !intervalSpecified(targetInterval) {
			continue
		}
		constraints = append(constraints, lpComponentConstraint{
			key:    key,
			coeffs: pair,
			target: targetInterval,
		})
	}
	return constraints
}

// Solver solves ice cream formulation problems.
type Solver struct {
	Problem *Problem

	opts SolverOptions
}

// NewSolver creates a solver for the given problem using default options.
func NewSolver(p *Problem) (*Solver, error) {
	return NewSolverWithOptions(p, SolverOptions{})
}

// NewSolverWithOptions creates a solver using the provided options.
func NewSolverWithOptions(p *Problem, opts SolverOptions) (*Solver, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	opts = applySolverOptionDefaults(opts)
	return &Solver{
		Problem: p,
		opts:    opts,
	}, nil
}

func (s *Solver) solve(lpp *lpProblem, objective []float64) (float64, []float64, error) {
	return lpp.solveNLopt(objective, s.opts)
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

	componentValues      map[componentKey]*coeffPair
	componentConstraints []lpComponentConstraint
	podLo                []float64
	podHi                []float64
	pacLo                []float64
	pacHi                []float64

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

type lpComponentConstraint struct {
	key    componentKey
	coeffs *coeffPair
	target Interval
}

// buildLP creates the LP formulation using midpoints of ingredient composition intervals.
func (s *Solver) buildLP() *lpProblem {
	p := s.Problem
	n := len(p.slots)
	ids := p.IngredientIDs()
	names := p.IngredientNames()
	idIndex := make(map[IngredientID]int, n)
	for i, id := range ids {
		idIndex[id] = i
	}

	lpp := &lpProblem{
		n:                n,
		componentValues:  componentPairsFromSlots(p.slots),
		target:           p.Target,
		targetPOD:        p.Target.POD,
		targetPAC:        p.Target.PAC,
		orderConstraints: p.OrderConstraints,
		constraints:      p.Constraints,
		ids:              ids,
		names:            names,
		idToIndex:        idIndex,
		podLo:            make([]float64, n),
		podHi:            make([]float64, n),
		pacLo:            make([]float64, n),
		pacHi:            make([]float64, n),
		lower:            make([]float64, n),
		upper:            make([]float64, n),
	}
	lpp.componentConstraints = buildComponentConstraints(lpp.componentValues, p.Target)

	for i, slot := range p.slots {
		if slot.definition == nil {
			lpp.lower[i] = 0
			lpp.upper[i] = 1
			continue
		}
		profile := slot.lot.EffectiveProfile()

		pod := profile.PODInterval()
		pac := profile.PACInterval()
		lpp.podLo[i] = pod.Lo
		lpp.podHi[i] = pod.Hi
		lpp.pacLo[i] = pac.Lo
		lpp.pacHi[i] = pac.Hi

		bounds := slot.bounds
		if bounds.Lo == 0 && bounds.Hi == 0 {
			bounds = Range(0, 1)
		}
		lpp.lower[i] = bounds.Lo
		lpp.upper[i] = bounds.Hi
	}

	return lpp
}

// buildLPWithCoeffs creates an LP with specific coefficient values.

func (s *Solver) buildLPWithCoeffs(coeffs coefficientSet) *lpProblem {
	p := s.Problem
	n := len(p.slots)
	ids := s.Problem.IngredientIDs()
	names := s.Problem.IngredientNames()
	idIndex := make(map[IngredientID]int, n)
	for i, id := range ids {
		idIndex[id] = i
	}

	lpp := &lpProblem{
		n:                n,
		componentValues:  componentPairsFromCoeffs(coeffs, n),
		target:           p.Target,
		targetPOD:        p.Target.POD,
		targetPAC:        p.Target.PAC,
		orderConstraints: p.OrderConstraints,
		constraints:      p.Constraints,
		ids:              ids,
		names:            names,
		idToIndex:        idIndex,
		podLo:            make([]float64, n),
		podHi:            make([]float64, n),
		pacLo:            make([]float64, n),
		pacHi:            make([]float64, n),
		lower:            make([]float64, n),
		upper:            make([]float64, n),
	}
	lpp.componentConstraints = buildComponentConstraints(lpp.componentValues, p.Target)

	for i, slot := range p.slots {
		pod := coeffs.pod[i]
		pac := coeffs.pac[i]

		lpp.podLo[i] = pod
		lpp.podHi[i] = pod
		lpp.pacLo[i] = pac
		lpp.pacHi[i] = pac

		bounds := slot.bounds
		if bounds.Lo == 0 && bounds.Hi == 0 {
			bounds = Range(0, 1)
		}
		lpp.lower[i] = bounds.Lo
		lpp.upper[i] = bounds.Hi
	}

	return lpp
}

// Feasible checks if the problem has any feasible solution.
func (s *Solver) Feasible() (bool, error) {
	lpp := s.buildLP()

	// Use zero objective (any feasible point)
	objective := make([]float64, lpp.n)

	_, _, err := s.solve(lpp, objective)
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
	_, _, err := s.solve(lpp, objective)
	if err != nil {
		return bounds, nil // infeasible
	}
	bounds.Feasible = true

	// For each ingredient, find min and max
	for i := 0; i < n; i++ {
		// Minimize w_i
		minObj := make([]float64, n)
		minObj[i] = 1
		minVal, _, err := s.solve(lpp, minObj)
		if err != nil {
			return nil, fmt.Errorf("error finding min for %s: %w", names[i], err)
		}

		// Maximize w_i (minimize -w_i)
		maxObj := make([]float64, n)
		maxObj[i] = -1
		maxVal, _, err := s.solve(lpp, maxObj)
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
	_, x, err := s.solve(lpp, objective)
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
	sol.Achieved = components
	return sol
}

func sumComponents(weights []float64, slots []ingredientSlot) ConstituentComponents {
	var agg ConstituentComponents
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if i >= len(slots) {
			continue
		}
		accumulateProfile(&agg, slots[i].lot.EffectiveProfile(), w)
	}
	return agg
}

func (s *Solver) snapshotFromWeights(weights []float64, opts MixOptions) (BatchSnapshot, ProcessProperties, error) {
	if len(s.Problem.slots) == 0 {
		return BatchSnapshot{}, ProcessProperties{}, errors.New("problem has no ingredients")
	}
	components := make([]RecipeComponent, 0, len(weights))
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if i >= len(s.Problem.slots) {
			continue
		}
		components = append(components, RecipeComponent{
			Ingredient: s.Problem.slots[i].lot,
			MassKg:     w,
		})
	}
	if len(components) == 0 {
		return BatchSnapshot{}, ProcessProperties{}, errors.New("solution has no positive ingredient weights")
	}
	return BuildProperties(components, opts)
}
