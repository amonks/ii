"""Ground-truth physics/chemistry calculations for frozen-dessert mixes."""

from __future__ import annotations

from typing import Any, Dict, Mapping, Sequence, Tuple, Literal, overload
import math

import casadi as ca

from .constants import (
    K_F_WATER,
    MW_GLU,
    MW_FRU,
    MW_LAC,
    MW_SUCR,
    MW_SORBITOL,
    MW_ERYTHRITOL,
    MIX_DENSITY_KG_PER_L,
    PINT_L,
)
from .ingredients import Ingredient

Numeric = Any
SymbolicMixProperties = Dict[str, Numeric]
FloatMixProperties = Dict[str, float]


@overload
def build_properties(
    ingredient_keys: Sequence[str],
    weights: Sequence,
    ingredients: Mapping[str, Ingredient],
    *,
    temp_C: float,
    draw_temp_C: float,
    shear_rate: float,
    symbolic: Literal[True],
    overrun_cap: float | None = None,
) -> SymbolicMixProperties:
    ...


@overload
def build_properties(
    ingredient_keys: Sequence[str],
    weights: Sequence,
    ingredients: Mapping[str, Ingredient],
    *,
    temp_C: float,
    draw_temp_C: float,
    shear_rate: float,
    symbolic: Literal[False],
    overrun_cap: float | None = None,
) -> FloatMixProperties:
    ...


@overload
def build_properties(
    ingredient_keys: Sequence[str],
    weights: Sequence,
    ingredients: Mapping[str, Ingredient],
    *,
    temp_C: float,
    draw_temp_C: float,
    shear_rate: float,
    symbolic: bool,
    overrun_cap: float | None = None,
) -> FloatMixProperties | SymbolicMixProperties:
    ...


def _ops(symbolic: bool) -> Dict[str, Any]:
    """Return math primitives that work for float or CasADi symbolic."""

    if symbolic:
        return {
            "exp": ca.exp,
            "sqrt": ca.sqrt,
            "log": ca.log,
            "max": ca.fmax,
            "min": ca.fmin,
            "abs": ca.fabs,
            "inf": ca.inf,  # type: ignore[attr-defined]
        }
    return {
        "exp": math.exp,
        "sqrt": math.sqrt,
        "log": math.log,
        "max": max,
        "min": min,
        "abs": abs,
        "inf": math.inf,
    }


def component_sums(
    ingredient_keys: Sequence[str],
    weights: Sequence,
    ingredients: Mapping[str, Ingredient],
    *,
    symbolic: bool,
) -> Dict[str, Numeric]:
    """Aggregate composition terms for the mix."""

    ops = _ops(symbolic)
    totals: Dict[str, Numeric] = {
        "total": 0,
        "water": 0,
        "fat": 0,
        "trans_fat": 0,
        "saturated_fat": 0,
        "saturated_fat_min": 0,
        "saturated_fat_max": 0,
        "protein": 0,
        "lactose": 0,
        "lactose_min": 0,
        "lactose_max": 0,
        "sucrose": 0,
        "glucose": 0,
        "fructose": 0,
        "maltodextrin": 0,
        "polyols": 0,
        "ash": 0,
        "other_solids": 0,
        "emulsifier_power": 0,
        "bound_water": 0,
        "polymer_solids": 0,
        "colligative_moles": 0,
        "cholesterol_mg": 0,
        "added_sugars": 0,
        "added_sugars_min": 0,
        "added_sugars_max": 0,
    }

    for i, key in enumerate(ingredient_keys):
        weight = weights[i]
        ing = ingredients[key]
        totals["total"] += weight
        totals["water"] += weight * ing.water
        totals["fat"] += weight * ing.fat
        totals["trans_fat"] += weight * ing.trans_fat
        sat_unit = ing.saturated_fat if ing.saturated_fat is not None else ing.fat
        sat_min_unit = ing.saturated_fat_min if ing.saturated_fat_min is not None else sat_unit
        sat_max_unit = ing.saturated_fat_max if ing.saturated_fat_max is not None else sat_unit
        totals["saturated_fat"] += weight * sat_unit
        totals["saturated_fat_min"] += weight * sat_min_unit
        totals["saturated_fat_max"] += weight * sat_max_unit
        totals["protein"] += weight * ing.protein
        lactose_unit = ing.lactose
        lactose_min_unit = ing.lactose_min if ing.lactose_min is not None else lactose_unit
        lactose_max_unit = ing.lactose_max if ing.lactose_max is not None else lactose_unit
        totals["lactose"] += weight * lactose_unit
        totals["lactose_min"] += weight * lactose_min_unit
        totals["lactose_max"] += weight * lactose_max_unit
        totals["sucrose"] += weight * ing.sucrose
        totals["glucose"] += weight * ing.glucose
        totals["fructose"] += weight * ing.fructose
        totals["maltodextrin"] += weight * ing.maltodextrin
        totals["polyols"] += weight * ing.polyols
        totals["ash"] += weight * ing.ash
        totals["other_solids"] += weight * ing.other_solids
        totals["emulsifier_power"] += weight * ing.emulsifier_power
        totals["bound_water"] += weight * ing.water_binding
        if ing.hydrocolloid:
            totals["polymer_solids"] += weight * (
                ing.other_solids + ing.maltodextrin + ing.polyols
            )
        if ing.cholesterol_mg_per_kg:
            totals["cholesterol_mg"] += weight * ing.cholesterol_mg_per_kg
        if ing.added_sugars or ing.added_sugars_min or ing.added_sugars_max:
            added_unit = ing.added_sugars
            added_min_unit = ing.added_sugars_min if ing.added_sugars_min is not None else added_unit
            added_max_unit = ing.added_sugars_max if ing.added_sugars_max is not None else added_unit
            totals["added_sugars"] += weight * added_unit
            totals["added_sugars_min"] += weight * added_min_unit
            totals["added_sugars_max"] += weight * added_max_unit

        maltodextrin_mw = MW_GLU * max(1.0, ing.maltodextrin_dp)
        moles = (
            weight * ing.sucrose * 1000.0 / MW_SUCR
            + weight * ing.glucose * 1000.0 / MW_GLU
            + weight * ing.fructose * 1000.0 / MW_FRU
            + weight * ing.lactose * 1000.0 / MW_LAC
            + weight * ing.maltodextrin * 1000.0 / maltodextrin_mw
            + weight * ing.polyols * 1000.0 / ing.polyol_mw
        )
        polymer_moles = weight * ing.other_solids * 1000.0 / ing.effective_mw
        totals["colligative_moles"] += (moles + polymer_moles) * ing.osmotic_coeff * ing.vh_factor

    return totals


def sweetness_eq(totals: Mapping[str, Numeric]) -> Numeric:
    """Return sucrose-equivalent mass (kg)."""

    return (
        totals["sucrose"] * 1.0
        + totals["glucose"] * 0.74
        + totals["fructose"] * 1.7
        + totals["lactose"] * 0.16
        + totals["maltodextrin"] * 0.20
        + totals["polyols"] * 0.60
    )


def freezing_point_and_ice_fraction(
    totals: Mapping[str, Numeric],
    *,
    temp_C: float,
    symbolic: bool,
) -> Tuple[Numeric, Numeric]:
    """Compute freezing point depression and ice fraction at a temperature."""

    ops = _ops(symbolic)
    water_available = ops["max"](1e-6, totals["water"] - totals["bound_water"])
    m_colligative = totals["colligative_moles"] / water_available
    freezing_point = -K_F_WATER * m_colligative

    abs_t = ops["abs"](temp_C)
    target_free_water = ops["max"](1e-6, totals["colligative_moles"] * K_F_WATER / ops["max"](1e-6, abs_t))
    target_free_water = ops["min"](target_free_water, water_available)
    ice_fraction = ops["max"](0.0, (water_available - target_free_water) / totals["water"])
    return freezing_point, ice_fraction


def viscosity(
    totals: Mapping[str, Numeric],
    *,
    temp_C: float,
    shear_rate: float,
    symbolic: bool,
) -> Numeric:
    """Estimate apparent viscosity (Pa·s) with a simple solids/polymer model."""

    ops = _ops(symbolic)
    solids_pct = (totals["total"] - totals["water"]) / totals["total"]
    polymer_pct = totals["polymer_solids"] / totals["total"]

    mu_serum = 0.0016 * ops["exp"](0.045 * (solids_pct * 100 - 36.0))
    polymer_factor = ops["exp"](12.0 * polymer_pct)
    temp_factor = ops["exp"](0.025 * (5.0 - temp_C))
    n = ops["max"](0.55, 1.0 - 0.6 * polymer_pct * 100)  # shear-thinning index
    mu = mu_serum * polymer_factor * temp_factor * (shear_rate / 50.0) ** (n - 1.0)
    return mu


def overrun(
    totals: Mapping[str, Numeric],
    *,
    viscosity_value: Numeric,
    symbolic: bool,
    cap: float | None = None,
) -> Numeric:
    """Predict overrun fraction from viscosity and fat destabilization heuristics.

    The returned value is optionally capped to represent freezer overrun settings.
    """

    ops = _ops(symbolic)
    fat_pct = totals["fat"] / totals["total"]
    protein = totals["protein"] / totals["total"]
    emulsifier = totals["emulsifier_power"] / totals["total"]
    destab_index = (fat_pct * 100.0) * (0.4 + emulsifier) / (4.0 + protein * 100.0)

    # Retuned sigmoids: shift viscosity knee upward to reflect packaged ice cream,
    # and moderate fat/emulsifier influence.
    visc_term = 1.0 / (1.0 + ops["exp"](6.5 * (viscosity_value - 0.45)))
    fat_term = 1.0 / (1.0 + ops["exp"](-3.0 * (destab_index - 1.2)))
    raw = ops["max"](0.02, ops["min"](1.1, 0.20 + 0.45 * fat_term + 0.35 * visc_term))
    if cap is not None:
        raw = ops["min"](raw, cap)
    return raw


def hardness_meltdown(
    *,
    ice_fraction: Numeric,
    solids_pct: Numeric,
    polyols: Numeric,
    overrun_value: Numeric,
    symbolic: bool,
) -> Tuple[Numeric, Numeric]:
    """Crude hardness and meltdown stability indices (dimensionless)."""

    ops = _ops(symbolic)
    hardness = 30.0 * ice_fraction + 8.0 * solids_pct + 3.0 * polyols
    meltdown = ops["max"](0.0, 1.2 * solids_pct + 0.8 * ice_fraction + 0.3 * overrun_value - 0.1 * polyols)
    return hardness, meltdown


def lactose_supersaturation(
    totals: Mapping[str, Numeric],
    *,
    temp_C: float,
    symbolic: bool,
) -> Numeric:
    """Estimate lactose supersaturation index vs. solubility curve."""

    ops = _ops(symbolic)
    solubility = 0.18 * ops["exp"](0.012 * temp_C + 1.2)
    available_water = ops["max"](1e-6, totals["water"] - totals["bound_water"])
    lact_conc = totals["lactose"] / available_water
    return lact_conc / ops["max"](1e-6, solubility)


def freezer_load(
    totals: Mapping[str, Numeric],
    *,
    draw_temp: float,
    ice_fraction: Numeric,
    symbolic: bool,
) -> Numeric:
    """Approximate freezer heat load in kJ."""

    cp = 3.4 - 1.2 * (totals["fat"] / totals["total"])
    delta_T = 4.0 - draw_temp
    latent = 333.0 * ice_fraction * totals["water"]
    return cp * totals["total"] * delta_T + latent


def build_properties(
    ingredient_keys: Sequence[str],
    weights: Sequence,
    ingredients: Mapping[str, Ingredient],
    *,
    temp_C: float,
    draw_temp_C: float,
    shear_rate: float,
    symbolic: bool,
    overrun_cap: float | None = None,
) -> FloatMixProperties | SymbolicMixProperties:
    """Calculate properties; returns dictionary for floats or symbolic expressions."""

    ops = _ops(symbolic)
    totals = component_sums(ingredient_keys, weights, ingredients, symbolic=symbolic)
    safe_total = ops["max"](1e-9, totals["total"])
    solids = totals["total"] - totals["water"]
    fat_pct = totals["fat"] / safe_total
    protein_pct = totals["protein"] / safe_total
    water_pct = totals["water"] / safe_total
    total_sugars = totals["sucrose"] + totals["glucose"] + totals["fructose"] + totals["lactose"]
    total_sugars_pct = total_sugars / safe_total
    sweetness = sweetness_eq(totals)
    trans_fat_pct = totals["trans_fat"] / safe_total
    saturated_fat_pct = totals["saturated_fat"] / safe_total
    saturated_fat_min_pct = totals["saturated_fat_min"] / safe_total
    saturated_fat_max_pct = totals["saturated_fat_max"] / safe_total
    lactose_pct = totals["lactose"] / safe_total
    lactose_min_pct = totals["lactose_min"] / safe_total
    lactose_max_pct = totals["lactose_max"] / safe_total
    added_sugars_pct = totals["added_sugars"] / safe_total
    added_sugars_min_pct = totals["added_sugars_min"] / safe_total
    added_sugars_max_pct = totals["added_sugars_max"] / safe_total
    cholesterol_mg_per_kg = totals["cholesterol_mg"] / safe_total
    freezing_point, ice_fraction = freezing_point_and_ice_fraction(
        totals, temp_C=temp_C, symbolic=symbolic
    )
    viscosity_value = viscosity(
        totals, temp_C=temp_C, shear_rate=shear_rate, symbolic=symbolic
    )
    overrun_value = overrun(
        totals, viscosity_value=viscosity_value, symbolic=symbolic, cap=overrun_cap
    )
    hardness, meltdown = hardness_meltdown(
        ice_fraction=ice_fraction,
        solids_pct=solids / totals["total"],
        polyols=totals["polyols"] / totals["total"],
        overrun_value=overrun_value,
        symbolic=symbolic,
    )
    lactose_ss = lactose_supersaturation(
        totals, temp_C=temp_C, symbolic=symbolic
    )
    load = freezer_load(
        totals, draw_temp=draw_temp_C, ice_fraction=ice_fraction, symbolic=symbolic
    )
    polymer_pct = totals["polymer_solids"] / safe_total

    cost_total = 0
    for i, key in enumerate(ingredient_keys):
        cost_total += weights[i] * ingredients[key].cost
    cost_per_kg = cost_total / totals["total"]

    mix_volume_L = totals["total"] / MIX_DENSITY_KG_PER_L
    pints_out = mix_volume_L * (1 + overrun_value) / PINT_L
    cost_per_pint = cost_total / pints_out

    result: Dict[str, Numeric] = {
        "total_mass": totals["total"],
        "water": totals["water"],
        "bound_water": totals["bound_water"],
        "fat": totals["fat"],
        "fat_pct": fat_pct,
        "trans_fat": totals["trans_fat"],
        "trans_fat_pct": trans_fat_pct,
        "saturated_fat": totals["saturated_fat"],
        "saturated_fat_pct": saturated_fat_pct,
        "saturated_fat_min": totals["saturated_fat_min"],
        "saturated_fat_min_pct": saturated_fat_min_pct,
        "saturated_fat_max": totals["saturated_fat_max"],
        "saturated_fat_max_pct": saturated_fat_max_pct,
        "protein": totals["protein"],
        "protein_pct": protein_pct,
        "water_pct": water_pct,
        "solids_pct": solids / safe_total,
        "total_sugars": total_sugars,
        "total_sugars_pct": total_sugars_pct,
        "lactose": totals["lactose"],
        "lactose_pct": lactose_pct,
        "lactose_min": totals["lactose_min"],
        "lactose_min_pct": lactose_min_pct,
        "lactose_max": totals["lactose_max"],
        "lactose_max_pct": lactose_max_pct,
        "solids": solids,
        "sweetness_eq": sweetness,
        "freezing_point": freezing_point,
        "ice_fraction_at_serve": ice_fraction,
        "viscosity_at_serve": viscosity_value,
        "overrun_estimate": overrun_value,
        "hardness_index": hardness,
        "meltdown_index": meltdown,
        "lactose_supersaturation": lactose_ss,
        "freezer_load_kj": load,
        "polymer_solids_pct": polymer_pct,
        "cholesterol_mg_total": totals["cholesterol_mg"],
        "cholesterol_mg_per_kg": cholesterol_mg_per_kg,
        "added_sugars": totals["added_sugars"],
        "added_sugars_pct": added_sugars_pct,
        "added_sugars_min": totals["added_sugars_min"],
        "added_sugars_min_pct": added_sugars_min_pct,
        "added_sugars_max": totals["added_sugars_max"],
        "added_sugars_max_pct": added_sugars_max_pct,
        "cost_total": cost_total,
        "cost_per_kg": cost_per_kg,
        "volume_L": mix_volume_L,
        "pints_yield": pints_out,
        "cost_per_pint_overrun": cost_per_pint,
    }

    if symbolic:
        return result

    return {k: float(v) for k, v in result.items()}


__all__ = [
    "component_sums",
    "sweetness_eq",
    "freezing_point_and_ice_fraction",
    "viscosity",
    "overrun",
    "hardness_meltdown",
    "lactose_supersaturation",
    "freezer_load",
    "build_properties",
]
