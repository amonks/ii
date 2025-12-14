"""CasADi/IPOPT formulation helpers for mix optimization problems."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, List, Mapping, MutableSequence, Sequence
import math

import casadi as ca

from .analysis import Recipe, ProductionSettings
from .chemistry import build_properties, SymbolicMixProperties
from .ingredients import Ingredient


@dataclass(frozen=True)
class FormulationGoals:
    """Desired mix goals expressed as mass fractions (0–1) and setpoints."""

    batch_mass: float = 100.0  # kg
    fat_pct: float = 0.12
    solids_pct: float = 0.38
    sweetness_pct: float = 0.16  # sucrose-equivalent mass fraction
    freezing_point_C: float = -3.4
    overrun: float = 0.85  # desired overrun (used only as upper target)
    overrun_cap: float | None = None  # operational cap; None means no cap
    serve_temperature_C: float = -12.0
    draw_temperature_C: float = -5.0
    shear_rate_s: float = 50.0  # s⁻¹ for viscosity estimation
    lactose_supersat_max: float = 1.1  # sandiness guardrail
    cost_weight: float = 0.05  # objective weight on cost minimisation


@dataclass(frozen=True)
class GoalWeights:
    """Relative weights for the least-squares objective."""

    fat: float = 900.0
    solids: float = 650.0
    sweetness: float = 450.0
    freezing_point: float = 220.0
    overrun: float = 180.0
    quality_viscosity: float = 12.0
    quality_meltdown: float = 8.0
    quality_hardness: float = 6.0
    quality_polymer: float = 5.0
    quality_lactose: float = 10.0
    quality_freezer_load: float = 4.0


@dataclass(frozen=True)
class Constraint:
    """Bounded constraint: lower ≤ expr ≤ upper."""

    expr: Any
    lower: float
    upper: float
    note: str | None = None


@dataclass(frozen=True)
class FormulationSolution:
    """Optimized ingredient weights and resulting properties."""

    weights: Dict[str, float]
    recipe: Recipe
    solver_status: str
    diagnostics: Dict[str, object] | None = None


class FormulationProblem:
    """Lightweight DSL for building and solving formulation problems."""

    def __init__(
        self,
        *,
        ingredients: Sequence[Ingredient],
        goals: FormulationGoals,
        bounds: Mapping[str, tuple[float, float]] | None = None,
        weights: GoalWeights | None = None,
    ) -> None:
        item_list: List[tuple[str, Ingredient]] = []
        if isinstance(ingredients, Mapping):
            raise TypeError("FormulationProblem now accepts only a sequence of Ingredient objects.")
        seen: set[str] = set()
        for entry in ingredients:
            if not isinstance(entry, Ingredient):
                raise TypeError("Ingredient sequences must contain Ingredient objects.")
            if entry.name in seen:
                raise ValueError(f"Duplicate ingredient name '{entry.name}' in sequence.")
            seen.add(entry.name)
            item_list.append((entry.name, entry))
        if not item_list:
            raise ValueError("At least one ingredient is required.")
        self.ingredient_keys = [name for name, _ in item_list]
        self.ingredients: Dict[str, Ingredient] = {name: ing for name, ing in item_list}
        self.goals = goals
        self.goal_weights = weights or GoalWeights()

        self._index = {k: i for i, k in enumerate(self.ingredient_keys)}
        self.x = ca.MX.sym("w", len(self.ingredient_keys))
        self._lbx: List[float] = []
        self._ubx: List[float] = []
        for key in self.ingredient_keys:
            if bounds is None:
                lo, hi = 0.0, goals.batch_mass
            else:
                lo, hi = bounds.get(key, (0.0, goals.batch_mass))
            self._lbx.append(lo)
            self._ubx.append(hi)

        self.props: SymbolicMixProperties = build_properties(
            self.ingredient_keys,
            self.x,
            self.ingredients,
            temp_C=self.goals.serve_temperature_C,
            draw_temp_C=self.goals.draw_temperature_C,
            shear_rate=self.goals.shear_rate_s,
            symbolic=True,
            overrun_cap=self.goals.overrun_cap,
        )
        self.constraints: List[Constraint] = []
        self._objective_expr = self._build_objective()
        self._add_default_constraints()

    # Convenience accessors -------------------------------------------------
    def expr(self, key: str) -> Any:
        return self.x[self._index[key]]

    def sum_of(self, keys: Iterable[str]) -> Any:
        return ca.sum1(ca.vcat([self.expr(k) for k in keys]))

    @property
    def fat_pct(self) -> Any:
        return self.props["fat"] / self.props["total_mass"]

    @property
    def solids_pct(self) -> Any:
        return self.props["solids"] / self.props["total_mass"]

    @property
    def sweetness_pct(self) -> Any:
        return self.props["sweetness_eq"] / self.props["total_mass"]

    @property
    def overrun(self) -> Any:
        return self.props["overrun_estimate"]

    # Objective and defaults -----------------------------------------------
    def _build_objective(self) -> Any:
        w = self.goal_weights
        props = self.props

        def band_penalty(value: Any, lo: float, hi: float) -> Any:
            return ca.fmax(0, lo - value) ** 2 + ca.fmax(0, value - hi) ** 2

        quality_penalty = 0.0
        quality_penalty += w.quality_viscosity * band_penalty(props["viscosity_at_serve"], 0.0025, 0.0065)
        quality_penalty += w.quality_meltdown * band_penalty(props["meltdown_index"], 0.8, 1.6)
        quality_penalty += w.quality_hardness * band_penalty(props["hardness_index"], 18.0, 36.0)
        quality_penalty += w.quality_polymer * ca.fmax(0, props["polymer_solids_pct"] - 0.012) ** 2
        quality_penalty += w.quality_lactose * ca.fmax(0, props["lactose_supersaturation"] - self.goals.lactose_supersat_max) ** 2
        quality_penalty += w.quality_freezer_load * ca.fmax(0, (props["freezer_load_kj"] - 19000.0) / 1000.0) ** 2

        return (
            w.fat * (self.fat_pct - self.goals.fat_pct) ** 2
            + w.solids * (self.solids_pct - self.goals.solids_pct) ** 2
            + w.sweetness * (self.sweetness_pct - self.goals.sweetness_pct) ** 2
            + w.freezing_point * (props["freezing_point"] - self.goals.freezing_point_C) ** 2
            # One-sided overrun: only penalise if predicted maximum overrun is below desired.
            + w.overrun * ca.fmax(0, self.goals.overrun - self.overrun) ** 2
            + quality_penalty
            + self.goals.cost_weight * props["cost_per_kg"]
        )

    def _add_default_constraints(self) -> None:
        # Mass balance
        self.add_constraint(self.props["total_mass"] - self.goals.batch_mass, 0.0, 0.0, "batch_mass")
        # Respect per-ingredient max_pct when present.
        # (callers can add their own ingredient-specific caps if needed)

    # Public mutation ------------------------------------------------------
    def add_constraint(self, expr: Any, lower: float, upper: float, note: str | None = None) -> None:
        self.constraints.append(Constraint(expr, lower, upper, note))

    def add_ordering(self, ordered_groups: Sequence[Sequence[str]], epsilon: float = 1e-3) -> None:
        """Enforce descending weight of aggregate groups (label order)."""

        for earlier, later in zip(ordered_groups, ordered_groups[1:]):
            w_earlier = self.sum_of(earlier)
            w_later = self.sum_of(later)
            self.add_constraint(w_earlier - w_later, epsilon, math.inf, "label_order")

    def add_soft_penalty(self, expr: Any, weight: float = 1.0) -> None:
        """Inject an additional quadratic (or arbitrary) penalty into the objective."""

        self._objective_expr = self._objective_expr + weight * expr

    def lock_macros(self, tolerance: float = 1e-4, *, include_overrun: bool = True, include_fp: bool = True) -> None:
        """Convert macro goals into hard constraints with small tolerance."""

        self.add_constraint(self.fat_pct - self.goals.fat_pct, -tolerance, tolerance, "fat_pct")
        self.add_constraint(self.solids_pct - self.goals.solids_pct, -tolerance, tolerance, "solids_pct")
        self.add_constraint(self.sweetness_pct - self.goals.sweetness_pct, -tolerance, tolerance, "sweetness_pct")
        if include_fp:
            self.add_constraint(
                self.props["freezing_point"] - self.goals.freezing_point_C, -tolerance, tolerance, "freezing_point"
            )
        # Overrun is not a hard equality; do not add a symmetric constraint.

    # Solver ---------------------------------------------------------------
    def solve(self) -> FormulationSolution:
        g: MutableSequence[Any] = []
        lbg: MutableSequence[float] = []
        ubg: MutableSequence[float] = []
        for c in self.constraints:
            g.append(c.expr)
            lbg.append(c.lower)
            ubg.append(c.upper)

        if g:
            constraint_func = ca.Function("constraint_eval", [self.x], [ca.vertcat(*g)])
        else:
            constraint_func = None

        nlp = {"x": self.x, "f": self._objective_expr, "g": ca.vertcat(*g)}
        opts = {
            "ipopt": {
                "print_level": 0,
                "sb": "yes",
                "max_iter": 500,
                "tol": 1e-8,
            },
            "print_time": False,
        }

        solver = ca.nlpsol("solver", "ipopt", nlp, opts)
        x0 = self._initial_guess()
        sol = solver(x0=x0, lbx=self._lbx, ubx=self._ubx, lbg=lbg, ubg=ubg)

        weights = {k: float(v) for k, v in zip(self.ingredient_keys, sol["x"].full().flatten().tolist())}
        weight_vector = sol["x"].full().flatten().tolist()
        ingredient_objs = [self.ingredients[key] for key in self.ingredient_keys]
        metrics = build_properties(
            self.ingredient_keys,
            weight_vector,
            self.ingredients,
            temp_C=self.goals.serve_temperature_C,
            draw_temp_C=self.goals.draw_temperature_C,
            shear_rate=self.goals.shear_rate_s,
            symbolic=False,
            overrun_cap=self.goals.overrun_cap,
        )
        metrics = {k: float(v) for k, v in metrics.items()}
        overrun_ceiling = metrics["overrun_estimate"]
        actual_overrun = min(self.goals.overrun, overrun_ceiling)
        metrics["overrun_actual"] = actual_overrun
        recipe = Recipe.from_weights(ingredient_objs, weight_vector, overrun=actual_overrun)
        snapshot = ProductionSettings(
            serve_temp_C=self.goals.serve_temperature_C,
            draw_temp_C=self.goals.draw_temperature_C,
            shear_rate_s=self.goals.shear_rate_s,
            overrun_cap=self.goals.overrun_cap,
            metrics=metrics,
        )
        recipe = recipe.with_mix_snapshot(snapshot)

        constraint_report: list[dict[str, float | str | None]] = []
        if constraint_func is not None:
            values = constraint_func(sol["x"]).full().flatten().tolist()
            for value, lower, upper, constraint in zip(values, lbg, ubg, self.constraints):
                violation = 0.0
                if upper < math.inf and value > upper:
                    violation = value - upper
                elif lower > -math.inf and value < lower:
                    violation = lower - value
                constraint_report.append(
                    {
                        "note": constraint.note,
                        "value": float(value),
                        "lower": float(lower),
                        "upper": float(upper),
                        "violation": float(violation),
                    }
                )

        diagnostics = {
            "iterations": solver.stats().get("iter_count", None),
            "constraint_report": constraint_report,
        }

        return FormulationSolution(
            weights=weights,
            recipe=recipe,
            solver_status=str(solver.stats().get("return_status", "unknown")),
            diagnostics=diagnostics,
        )

    def _initial_guess(self) -> ca.DM:
        """Uniform initial guess scaled to batch mass."""

        base = [self.goals.batch_mass / max(1, len(self.ingredient_keys))] * len(self.ingredient_keys)
        return ca.DM(base)


def solve_formulation(
    goals: FormulationGoals,
    *,
    ingredients: Sequence[Ingredient],
    constraints: Sequence[Constraint] | None = None,
    order_groups: Sequence[Sequence[str]] | None = None,
    weights: GoalWeights | None = None,
) -> FormulationSolution:
    """Convenience wrapper when callers do not need the DSL directly."""

    problem = FormulationProblem(ingredients=ingredients, goals=goals, weights=weights)
    if order_groups:
        problem.add_ordering(order_groups)
    if constraints:
        for c in constraints:
            problem.add_constraint(c.expr, c.lower, c.upper, c.note)
    return problem.solve()


__all__ = [
    "FormulationGoals",
    "GoalWeights",
    "Constraint",
    "FormulationSolution",
    "FormulationProblem",
    "solve_formulation",
]
