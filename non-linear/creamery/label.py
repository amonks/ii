"""Helpers for building formulation problems from CPG-style ingredient labels."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Mapping, Sequence
import math

from .analysis import NutritionFacts
from .constants import K_F_WATER, MW_SUCR, MIX_DENSITY_KG_PER_L, PINT_L
from .model import (
    Constraint,
    FormulationGoals,
    FormulationProblem,
    FormulationSolution,
)


@dataclass(frozen=True)
class LabelGroup:
    """A label line; keys are ingredient variables aggregated for ordering."""

    name: str
    keys: Sequence[str]
    fraction_bounds: Mapping[str, tuple[float, float]] = field(default_factory=dict)
    enforce_internal_order: bool = False


def goals_from_label(
    facts: NutritionFacts,
    pint_mass_g: float,
    *,
    serve_temp_C: float = -12.0,
    draw_temp_C: float = -5.0,
    shear_rate_s: float = 50.0,
) -> FormulationGoals:
    """Convert a nutrition panel and pint mass into target mix goals."""

    mass_kg = pint_mass_g / 1000.0
    servings = pint_mass_g / facts.serving_size_g
    fat = facts.total_fat_g * servings / 1000.0
    carbs = facts.total_carbs_g * servings / 1000.0
    sugars = facts.total_sugars_g * servings / 1000.0
    protein = facts.protein_g * servings / 1000.0
    ash = (facts.sodium_mg * servings / 1000.0) * (58.44 / 23.0) / 1000.0  # Na→NaCl kg
    solids = fat + carbs + protein + ash
    water = max(1e-6, mass_kg - solids)

    sucrose_moles = sugars * 1000.0 / MW_SUCR
    fp_est = -K_F_WATER * sucrose_moles / water
    finished_density = mass_kg / PINT_L
    overrun_guess = max(0.0, min(1.5, MIX_DENSITY_KG_PER_L / finished_density - 1.0))

    return FormulationGoals(
        batch_mass=100.0,
        fat_pct=fat / mass_kg,
        solids_pct=solids / mass_kg,
        sweetness_pct=sugars / mass_kg,
        freezing_point_C=float(fp_est),
        overrun=overrun_guess,
        serve_temperature_C=serve_temp_C,
        draw_temperature_C=draw_temp_C,
        shear_rate_s=shear_rate_s,
    )


def apply_group_bounds(problem: FormulationProblem, groups: Sequence[LabelGroup]) -> None:
    """Add within-group fraction bounds (e.g., cream fat share, liquid sugar solids)."""

    for group in groups:
        group_total = problem.sum_of(group.keys)
        for key, (lo, hi) in group.fraction_bounds.items():
            # If the group has a single member, fraction bounds are meaningless (always 1.0).
            if len(group.keys) == 1 and group.keys[0] == key:
                continue
            problem.add_constraint(problem.expr(key) - hi * group_total, -math.inf, 0.0, f"{group.name}:{key}:hi")
            problem.add_constraint(problem.expr(key) - lo * group_total, 0.0, math.inf, f"{group.name}:{key}:lo")


def apply_label_order(problem: FormulationProblem, groups: Sequence[LabelGroup], epsilon: float = 1e-3) -> None:
    """Enforce descending label order for groups and within-group members."""

    ordered = [g.keys for g in groups]
    problem.add_ordering(ordered, epsilon=epsilon)

    for group in groups:
        if group.enforce_internal_order and len(group.keys) > 1:
            within = [[key] for key in group.keys]
            problem.add_ordering(within, epsilon=epsilon)


__all__ = [
    "LabelGroup",
    "goals_from_label",
    "apply_group_bounds",
    "apply_label_order",
]
