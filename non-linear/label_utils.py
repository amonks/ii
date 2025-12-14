"""Shared reporting utilities for label-driven formulation scenarios."""

from __future__ import annotations

from typing import Iterable, Mapping, Sequence, Tuple

from creamery import Ingredient, Recipe
from creamery.constants import SERVING_PORTION_L
from creamery.model import FormulationGoals, FormulationSolution

_COMPONENT_ATTRS = [
    "water",
    "fat",
    "trans_fat",
    "saturated_fat",
    "protein",
    "lactose",
    "sucrose",
    "glucose",
    "fructose",
    "maltodextrin",
    "polyols",
    "ash",
    "other_solids",
    "cost",
    "osmotic_coeff",
    "vh_factor",
    "water_binding",
    "effective_mw",
    "maltodextrin_dp",
    "polyol_mw",
    "emulsifier_power",
    "cholesterol_mg_per_kg",
    "added_sugars",
]
_MIN_COMPONENT_WEIGHT = 1e-8


def _blend_ingredient(
    name: str,
    component_weights: Mapping[str, float],
    ingredients: Mapping[str, Ingredient],
) -> Ingredient:
    total = sum(component_weights.values())
    if total <= _MIN_COMPONENT_WEIGHT:
        raise ValueError("Cannot blend ingredients with zero total mass")

    def weighted(attr: str) -> float:
        return (
            sum(component_weights[k] * getattr(ingredients[k], attr) for k in component_weights)
            / total
        )

    blended = {attr: weighted(attr) for attr in _COMPONENT_ATTRS}
    hydrocolloid = any(ingredients[k].hydrocolloid for k in component_weights if component_weights[k] > 0)
    return Ingredient(name=name, hydrocolloid=hydrocolloid, **blended)


def recipe_from_solution(
    solution: FormulationSolution,
    ingredient_keys: Sequence[str],
    ingredients: Mapping[str, Ingredient],
    goals: FormulationGoals,
) -> Tuple[Recipe, float]:
    """Convert solver weights into a display recipe and derived serving size."""

    base_weights = [float(solution.weights[k]) for k in ingredient_keys]
    total_mass = sum(base_weights)
    if total_mass <= _MIN_COMPONENT_WEIGHT:
        raise ValueError("Solution has no mass to convert into a recipe")

    remaining = {k: w for k, w in zip(ingredient_keys, base_weights)}
    entries: list[tuple[Ingredient, float]] = []

    def promote_group(keys: Iterable[str], name_builder):
        component_weights = {}
        for key in keys:
            weight = remaining.get(key, 0.0)
            if weight > _MIN_COMPONENT_WEIGHT:
                component_weights[key] = weight
        if len(component_weights) < 2:
            return None
        total = sum(component_weights.values())
        if total <= _MIN_COMPONENT_WEIGHT:
            return None
        name = name_builder(component_weights)
        blended = _blend_ingredient(name, component_weights, ingredients)
        for key in component_weights:
            remaining.pop(key, None)
        return blended, total

    def add_entry(ingredient_obj: Ingredient, weight: float) -> None:
        if weight <= _MIN_COMPONENT_WEIGHT:
            return
        entries.append((ingredient_obj, weight))

    def cream_label(component_weights: Mapping[str, float]) -> str:
        total = sum(component_weights.values())
        fat_mass = sum(component_weights[k] * ingredients[k].fat for k in component_weights)
        fat_pct = fat_mass / total if total > 0 else 0.0
        return f"cream ({fat_pct*100:.1f}% fat)"

    def liquid_sugar_label(component_weights: Mapping[str, float]) -> str:
        total = sum(component_weights.values())
        water_mass = sum(component_weights[k] * ingredients[k].water for k in component_weights)
        solids_pct = max(0.0, 1.0 - water_mass / total) if total > 0 else 0.0
        return f"liquid sugar ({solids_pct*100:.1f}% solids)"

    composite_entries = [
        promote_group(["cream_fat", "cream_serum"], cream_label),
        promote_group(["liquid_sugar_sucrose", "liquid_sugar_water"], liquid_sugar_label),
    ]
    for entry in composite_entries:
        if entry is not None:
            add_entry(*entry)

    for key in ingredient_keys:
        weight = remaining.pop(key, 0.0)
        ingredient_obj = ingredients[key]
        add_entry(ingredient_obj, weight)

    display_recipe = Recipe(entries, overrun=solution.recipe.overrun)
    if solution.recipe.mix_snapshot is not None:
        display_recipe = display_recipe.with_mix_snapshot(solution.recipe.mix_snapshot)

    mix_recipe = solution.recipe
    serving_size_g = mix_recipe.serving_size_for_volume(
        SERVING_PORTION_L,
        serve_temp_C=goals.serve_temperature_C,
        draw_temp_C=goals.draw_temperature_C,
        shear_rate_s=goals.shear_rate_s,
        overrun_cap=goals.overrun_cap,
    )

    return display_recipe, serving_size_g


__all__ = ["recipe_from_solution"]
