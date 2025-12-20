"""CLI helper to analyse a fixed mix recipe and dump every available metric."""

from __future__ import annotations

import math
from typing import Sequence, Tuple

from creamery import Recipe, default_ingredients
from creamery.analysis import ProductionSettings
from creamery.constants import SERVING_PORTION_L
from creamery.chemistry import build_properties
from creamery.model import FormulationGoals

Component = Tuple[str, float, str]

RAW_COMPONENTS: Sequence[Component] = (
    ("cream36", 0.432, "Cream (36%)"),
    ("whole_milk", 0.267, "Whole milk (3.25%)"),
    ("skim_milk_powder", 0.112, "Skim milk powder"),
    ("sucrose", 0.190, "Sucrose"),
)

BATCH_MASS_KG = 100.0


def _normalize_components(components: Sequence[Component]) -> Sequence[Component]:
    total = sum(fraction for _, fraction, _ in components)
    if total <= 0:
        raise ValueError("Recipe fractions must sum to a positive value.")
    return tuple((key, fraction / total, label) for key, fraction, label in components)


def _build_recipe(goals: FormulationGoals) -> tuple[Recipe, dict[str, float], Sequence[Component]]:
    normalized = _normalize_components(RAW_COMPONENTS)
    table = default_ingredients()
    keys = []
    weights = []
    ingredient_objs = []
    for key, fraction, _ in normalized:
        if key not in table:
            raise KeyError(f"Ingredient '{key}' is not defined in the default pantry.")
        keys.append(key)
        weights.append(fraction * goals.batch_mass)
        ingredient_objs.append(table[key])
    recipe = Recipe.from_weights(ingredient_objs, weights, overrun=0.0)
    metrics = build_properties(
        keys,
        weights,
        {key: table[key] for key in keys},
        temp_C=goals.serve_temperature_C,
        draw_temp_C=goals.draw_temperature_C,
        shear_rate=goals.shear_rate_s,
        symbolic=False,
        overrun_cap=goals.overrun_cap,
    )
    overrun = metrics.get("overrun_estimate", 0.0)
    recipe = recipe.with_overrun(overrun)
    snapshot = ProductionSettings(
        serve_temp_C=goals.serve_temperature_C,
        draw_temp_C=goals.draw_temperature_C,
        shear_rate_s=goals.shear_rate_s,
        overrun_cap=goals.overrun_cap,
        metrics=metrics,
    )
    recipe = recipe.with_mix_snapshot(snapshot)
    return recipe, metrics, normalized


def _format_value(value: float) -> str:
    if math.isnan(value) or math.isinf(value):
        return str(value)
    magnitude = abs(value)
    if magnitude == 0:
        return "0"
    if magnitude >= 1000:
        return f"{value:,.2f}"
    if magnitude >= 1:
        return f"{value:.4f}"
    if magnitude >= 0.01:
        return f"{value:.6f}"
    return f"{value:.8f}"


def main() -> None:
    goals = FormulationGoals(
        batch_mass=BATCH_MASS_KG,
        overrun=0.45,
        serve_temperature_C=-12.0,
        draw_temperature_C=-5.0,
        shear_rate_s=50.0,
    )
    raw_total = sum(fraction for _, fraction, _ in RAW_COMPONENTS)
    recipe, metrics, normalized = _build_recipe(goals)
    fractions = recipe.fractions()
    serving_size_g = recipe.serving_size_for_volume(
        SERVING_PORTION_L,
        serve_temp_C=goals.serve_temperature_C,
        draw_temp_C=goals.draw_temperature_C,
        shear_rate_s=goals.shear_rate_s,
        overrun_cap=goals.overrun_cap,
    )
    facts = recipe.nutrition_facts(serving_size_g)
    formulation = recipe.formulation()

    print("=== Input Mix ===")
    for key, fraction, label in normalized:
        weight = fraction * goals.batch_mass
        display_name = f"{label} [{key}]"
        print(f"  - {display_name:<28s}{fraction*100:6.2f}% ({weight:6.2f} kg)")
    if not math.isclose(raw_total, 1.0, rel_tol=5e-4):
        print(f"  * Note: raw inputs summed to {raw_total*100:.2f}% and were renormalized for analysis.")

    print("\n=== Recipe Summary ===\n")
    print(recipe.__print__())

    print("\n=== Formulation ===\n")
    print(formulation.__print__())

    print("\n=== Nutrition Facts ===\n")
    print(facts.__print__())
    print(f"\n  Serving size used: {serving_size_g:.1f} g")

    print("\n=== Ingredient Fractions (by mass) ===")
    for name, fraction in sorted(fractions.items(), key=lambda kv: -kv[1]):
        print(f"  - {name:<20s}{fraction*100:6.2f}%")

    print("\n=== Raw Mix Metrics ===")
    for key in sorted(metrics):
        print(f"  {key:24s}: {_format_value(metrics[key])}")


if __name__ == "__main__":
    main()
