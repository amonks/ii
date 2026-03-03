package creamery

import (
	"math"
	"strings"

	"github.com/go-nlopt/nlopt"
)

func (lpp *lpProblem) solveNLopt(objective []float64, opts SolverOptions) (float64, []float64, error) {
	n := lpp.n
	objVec := make([]float64, n)
	copy(objVec, objective)
	return lpp.solveWithObjective(opts, func(x, grad []float64) float64 {
		if grad != nil {
			copy(grad, objVec)
		}
		return dot(objVec, x)
	})
}

func (lpp *lpProblem) solveWithObjective(opts SolverOptions, objective func(x, grad []float64) float64) (float64, []float64, error) {
	value, solution, err := lpp.tryOptimize(opts, objective)
	if err != nil && isRoundoffLimited(err) {
		// ROUNDOFF_LIMITED means the optimizer couldn't improve further
		// due to floating-point precision. Retry with relaxed tolerance,
		// which often succeeds. This mirrors what NLopt 2.8+ does
		// internally for SLSQP.
		relaxed := opts
		relaxed.ConstraintTolerance = opts.ConstraintTolerance * 10
		value, solution, err = lpp.tryOptimize(relaxed, objective)
	}
	return value, solution, err
}

func (lpp *lpProblem) tryOptimize(opts SolverOptions, objective func(x, grad []float64) float64) (float64, []float64, error) {
	opt, initial, err := lpp.configureOptimizer(opts)
	if err != nil {
		return 0, nil, err
	}
	defer opt.Destroy()

	if err := opt.SetMinObjective(objective); err != nil {
		return 0, nil, err
	}

	solution, value, err := opt.Optimize(initial)
	if err != nil {
		return 0, nil, err
	}
	return value, solution, nil
}

func isRoundoffLimited(err error) bool {
	return err != nil && strings.Contains(err.Error(), "ROUNDOFF_LIMITED")
}

func (lpp *lpProblem) configureOptimizer(opts SolverOptions) (*nlopt.NLopt, []float64, error) {
	n := lpp.n
	alg := opts.NLoptAlgorithm
	if alg == 0 {
		alg = defaultNLoptAlgorithm
	}
	opt, err := nlopt.NewNLopt(alg, uint(n))
	if err != nil {
		return nil, nil, err
	}

	success := false
	defer func() {
		if !success {
			opt.Destroy()
		}
	}()

	if err := opt.SetLowerBounds(append([]float64(nil), lpp.lower...)); err != nil {
		return nil, nil, err
	}
	if err := opt.SetUpperBounds(append([]float64(nil), lpp.upper...)); err != nil {
		return nil, nil, err
	}
	if opts.MaxEval > 0 {
		if err := opt.SetMaxEval(opts.MaxEval); err != nil {
			return nil, nil, err
		}
	}
	if err := opt.SetXtolRel(opts.ConstraintTolerance); err != nil {
		return nil, nil, err
	}

	tol := opts.ConstraintTolerance

	sumOnes := make([]float64, n)
	for i := range sumOnes {
		sumOnes[i] = 1
	}
	if err := addLinearEqualityConstraint(opt, sumOnes, -1, tol); err != nil {
		return nil, nil, err
	}

	for _, comp := range lpp.componentConstraints {
		if err := addLinearConstraint(opt, negateSlice(comp.coeffs.lo), comp.target.Lo, tol); err != nil {
			return nil, nil, err
		}
		if err := addLinearConstraint(opt, append([]float64(nil), comp.coeffs.hi...), -comp.target.Hi, tol); err != nil {
			return nil, nil, err
		}
	}

	if intervalSpecified(lpp.targetPOD) {
		if err := addLinearConstraint(opt, negateSlice(lpp.podLo), lpp.targetPOD.Lo, tol); err != nil {
			return nil, nil, err
		}
		if err := addLinearConstraint(opt, append([]float64(nil), lpp.podHi...), -lpp.targetPOD.Hi, tol); err != nil {
			return nil, nil, err
		}
	}
	if intervalSpecified(lpp.targetPAC) {
		if err := addLinearConstraint(opt, negateSlice(lpp.pacLo), lpp.targetPAC.Lo, tol); err != nil {
			return nil, nil, err
		}
		if err := addLinearConstraint(opt, append([]float64(nil), lpp.pacHi...), -lpp.targetPAC.Hi, tol); err != nil {
			return nil, nil, err
		}
	}

	if lpp.orderConstraints {
		for i := 0; i < n-1; i++ {
			coeff := make([]float64, n)
			coeff[i] = -1
			coeff[i+1] = 1
			if err := addLinearConstraint(opt, coeff, 0, tol); err != nil {
				return nil, nil, err
			}
		}
	}

	for _, constraint := range lpp.constraints {
		if constraint.Upper < math.Inf(1) {
			coeff := make([]float64, n)
			for id, v := range constraint.Coeffs {
				if idx, ok := lpp.idToIndex[id]; ok {
					coeff[idx] = v
				}
			}
			if err := addLinearConstraint(opt, coeff, -constraint.Upper, tol); err != nil {
				return nil, nil, err
			}
		}
		if constraint.Lower > math.Inf(-1) {
			coeff := make([]float64, n)
			for id, v := range constraint.Coeffs {
				if idx, ok := lpp.idToIndex[id]; ok {
					coeff[idx] = -v
				}
			}
			if err := addLinearConstraint(opt, coeff, constraint.Lower, tol); err != nil {
				return nil, nil, err
			}
		}
	}

	initial := lpp.initialGuess()
	success = true
	return opt, initial, nil
}

func addLinearConstraint(opt *nlopt.NLopt, coeff []float64, offset, tol float64) error {
	c := append([]float64(nil), coeff...)
	return opt.AddInequalityConstraint(func(x, grad []float64) float64 {
		if grad != nil {
			copy(grad, c)
		}
		return dot(c, x) + offset
	}, tol)
}

func addLinearEqualityConstraint(opt *nlopt.NLopt, coeff []float64, offset, tol float64) error {
	c := append([]float64(nil), coeff...)
	return opt.AddEqualityConstraint(func(x, grad []float64) float64 {
		if grad != nil {
			copy(grad, c)
		}
		return dot(c, x) + offset
	}, tol)
}

func negateSlice(values []float64) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = -v
	}
	return out
}

func dot(a, b []float64) float64 {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	sum := 0.0
	for i := 0; i < limit; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (lpp *lpProblem) initialGuess() []float64 {
	n := lpp.n
	guess := make([]float64, n)
	base := 1.0 / math.Max(1, float64(n))
	sum := 0.0
	for i := range n {
		guess[i] = clampFloat(base, lpp.lower[i], lpp.upper[i])
		sum += guess[i]
	}
	if sum <= 0 {
		sum = 1
	}
	scale := 1 / sum
	for i := range n {
		guess[i] = clampFloat(guess[i]*scale, lpp.lower[i], lpp.upper[i])
	}
	return guess
}
