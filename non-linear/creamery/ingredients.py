"""Ingredient facts: compositions, physical constants, and helper utilities."""

from __future__ import annotations

from dataclasses import dataclass, replace
from typing import Any, Dict, Mapping, Sequence, Tuple

from .constants import (
    MW_SUCR,
    MW_GLU,
    MW_FRU,
    MW_SORBITOL,
    MW_GLYCEROL,
    MW_ERYTHRITOL,
    LABEL_PERCENT_EPS,
)


DAIRY_TRANS_FAT_SHARE = 0.035
EGG_TRANS_FAT_SHARE = 0.01
DAIRY_SAT_FAT_RANGE = (0.60, 0.75)
EGG_SAT_FAT_RANGE = (0.30, 0.40)
PLANT_SAT_FAT_RANGE = (0.10, 0.20)
COCOA_SAT_FAT_RANGE = (0.50, 0.70)


def _mg_per_kg_from_100g(value: float) -> float:
    return value * 10.0


def _expand_fraction_bounds(lo: float, hi: float, *, eps: float = LABEL_PERCENT_EPS) -> tuple[float, float]:
    return (max(0.0, lo * (1 - eps)), min(1.0, hi * (1 + eps)))


@dataclass(frozen=True)
class Ingredient:
    """Ingredient composition and meta-properties on a per-kg basis."""

    name: str
    water: float
    fat: float = 0.0
    trans_fat: float = 0.0
    saturated_fat: float | None = None
    saturated_fat_min: float | None = None
    saturated_fat_max: float | None = None
    protein: float = 0.0
    lactose: float = 0.0
    lactose_min: float | None = None
    lactose_max: float | None = None
    sucrose: float = 0.0
    glucose: float = 0.0
    fructose: float = 0.0
    maltodextrin: float = 0.0
    polyols: float = 0.0
    ash: float = 0.0
    other_solids: float = 0.0
    cost: float = 0.0  # $/kg
    osmotic_coeff: float = 1.0  # non-ideality factor
    vh_factor: float = 1.0  # van't Hoff factor
    water_binding: float = 0.0  # kg water bound per kg ingredient
    effective_mw: float = MW_SUCR  # g/mol for "other_solids" colligative estimate
    maltodextrin_dp: float = 10.0  # degree of polymerization for maltodextrin bucket
    polyol_mw: float = MW_SORBITOL
    emulsifier_power: float = 0.0  # relative scale
    hydrocolloid: bool = False
    cholesterol_mg_per_kg: float = 0.0
    added_sugars: float = 0.0
    added_sugars_min: float | None = None
    added_sugars_max: float | None = None
    
    def clone_with(self, *, name: str | None = None, **overrides: Any) -> "Ingredient":
        params = dict(overrides)
        if name is not None:
            params["name"] = name
        return replace(self, **params)

    def clone(self, *, name: str | None = None, **overrides: Any) -> "Ingredient":
        """Backward-compatible alias for clone_with."""

        return self.clone_with(name=name, **overrides)


def _ingredient(
    name: str,
    water: float,
    *,
    fat: float = 0.0,
    trans_fat: float = 0.0,
    saturated_fat: float | None = None,
    saturated_fat_min: float | None = None,
    saturated_fat_max: float | None = None,
    protein: float = 0.0,
    lactose: float = 0.0,
    lactose_min: float | None = None,
    lactose_max: float | None = None,
    sucrose: float = 0.0,
    glucose: float = 0.0,
    fructose: float = 0.0,
    maltodextrin: float = 0.0,
    polyols: float = 0.0,
    ash: float = 0.0,
    other_solids: float = 0.0,
    cost: float = 0.0,
    osmotic_coeff: float = 1.0,
    vh_factor: float = 1.0,
    water_binding: float = 0.0,
    effective_mw: float = MW_SUCR,
    maltodextrin_dp: float = 10.0,
    polyol_mw: float = MW_SORBITOL,
    emulsifier_power: float = 0.0,
    hydrocolloid: bool = False,
    cholesterol_mg_per_kg: float = 0.0,
    added_sugars: float | None = None,
    added_sugars_min: float | None = None,
    added_sugars_max: float | None = None,
) -> Ingredient:
    """Helper to define ingredients concisely with sensible defaults."""

    if saturated_fat is None:
        saturated_fat = fat
    if saturated_fat_min is None:
        saturated_fat_min = saturated_fat
    if saturated_fat_max is None:
        saturated_fat_max = saturated_fat
    if lactose_min is None:
        lactose_min = lactose
    if lactose_max is None:
        lactose_max = lactose
    if added_sugars is None:
        added_sugars = 0.0
    if added_sugars_min is None:
        added_sugars_min = added_sugars
    if added_sugars_max is None:
        added_sugars_max = added_sugars

    return Ingredient(
        name=name,
        water=water,
        fat=fat,
        trans_fat=trans_fat,
        saturated_fat=saturated_fat,
        saturated_fat_min=saturated_fat_min,
        saturated_fat_max=saturated_fat_max,
        protein=protein,
        lactose=lactose,
        lactose_min=lactose_min,
        lactose_max=lactose_max,
        sucrose=sucrose,
        glucose=glucose,
        fructose=fructose,
        maltodextrin=maltodextrin,
        polyols=polyols,
        ash=ash,
        other_solids=other_solids,
        cost=cost,
        osmotic_coeff=osmotic_coeff,
        vh_factor=vh_factor,
        water_binding=water_binding,
        effective_mw=effective_mw,
        maltodextrin_dp=maltodextrin_dp,
        polyol_mw=polyol_mw,
        emulsifier_power=emulsifier_power,
        hydrocolloid=hydrocolloid,
        cholesterol_mg_per_kg=cholesterol_mg_per_kg,
        added_sugars=added_sugars,
        added_sugars_min=added_sugars_min,
        added_sugars_max=added_sugars_max,
    )


def default_ingredients() -> Dict[str, Ingredient]:
    """Return a fresh ingredient table so callers can extend/clone safely."""

    table = {
        "water": _ingredient("water", water=1.0, cost=0.0),
        "whole_milk": _ingredient(
            "whole_milk",
            water=0.873,
            fat=0.0325,
            trans_fat=0.0325 * DAIRY_TRANS_FAT_SHARE,
            protein=0.032,
            lactose=0.049,
            ash=0.0085,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(14.0),
        ),
        "cream_fat": _ingredient(
            "cream_fat",
            water=0.0,
            fat=1.0,
            trans_fat=1.0 * DAIRY_TRANS_FAT_SHARE,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(260.0),
        ),
        "cream_serum": _ingredient(
            "cream_serum",
            water=0.907,
            fat=0.0,
            protein=0.035,
            lactose=0.05,
            ash=0.008,
            cost=0.0,
        ),
        "skim_milk": _ingredient(
            "skim_milk",
            water=0.905,
            fat=0.002,
            trans_fat=0.002 * DAIRY_TRANS_FAT_SHARE,
            protein=0.035,
            lactose=0.05,
            ash=0.008,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(5.0),
        ),
        "heavy_cream": _ingredient(
            "heavy_cream",
            water=0.60,
            fat=0.36,
            trans_fat=0.36 * DAIRY_TRANS_FAT_SHARE,
            protein=0.02,
            lactose=0.015,
            ash=0.005,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(110.0),
        ),
        "whipping_cream": _ingredient(
            "whipping_cream",
            water=0.66,
            fat=0.30,
            trans_fat=0.30 * DAIRY_TRANS_FAT_SHARE,
            protein=0.02,
            lactose=0.015,
            ash=0.005,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(100.0),
        ),
        "light_cream": _ingredient(
            "light_cream",
            water=0.76,
            fat=0.18,
            trans_fat=0.18 * DAIRY_TRANS_FAT_SHARE,
            protein=0.03,
            lactose=0.02,
            ash=0.01,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(60.0),
        ),
        "cream36": _ingredient(
            "cream36",
            water=0.60,
            fat=0.36,
            trans_fat=0.36 * DAIRY_TRANS_FAT_SHARE,
            protein=0.02,
            lactose=0.03,
            ash=0.01,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(110.0),
        ),
        "anhydrous_milk_fat": _ingredient(
            "anhydrous_milk_fat",
            water=0.0,
            fat=0.999,
            trans_fat=0.999 * DAIRY_TRANS_FAT_SHARE,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(260.0),
        ),
        "butter": _ingredient(
            "butter",
            water=0.16,
            fat=0.80,
            trans_fat=0.80 * DAIRY_TRANS_FAT_SHARE,
            protein=0.01,
            lactose=0.02,
            ash=0.01,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(215.0),
        ),
        "skim_milk_powder": _ingredient(
            "skim_milk_powder",
            water=0.04,
            fat=0.01,
            trans_fat=0.01 * DAIRY_TRANS_FAT_SHARE,
            protein=0.35,
            lactose=0.49,
            ash=0.06,
            other_solids=0.05,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(150.0),
        ),
        "wpc80": _ingredient(
            "wpc80",
            water=0.05,
            fat=0.01,
            trans_fat=0.01 * DAIRY_TRANS_FAT_SHARE,
            protein=0.80,
            lactose=0.08,
            ash=0.06,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(180.0),
        ),
        "egg_yolk": _ingredient(
            "egg_yolk",
            water=0.52,
            fat=0.32,
            trans_fat=0.32 * EGG_TRANS_FAT_SHARE,
            protein=0.16,
            ash=0.01,
            cost=0.0,
            cholesterol_mg_per_kg=_mg_per_kg_from_100g(1085.0),
        ),
        "sucrose": _ingredient(
            "sucrose",
            water=0.0,
            sucrose=0.999,
            cost=0.0,
            effective_mw=MW_SUCR,
        ),
        "dextrose": _ingredient(
            "dextrose",
            water=0.01,
            glucose=0.99,
            cost=0.0,
            effective_mw=MW_GLU,
        ),
        "fructose": _ingredient(
            "fructose",
            water=0.01,
            fructose=0.99,
            cost=0.0,
            effective_mw=MW_FRU,
        ),
        "corn_syrup_42": _ingredient(
            "corn_syrup_42",
            water=0.20,
            glucose=0.15,
            fructose=0.05,
            maltodextrin=0.58,
            ash=0.02,
            cost=0.0,
            maltodextrin_dp=2.4,
        ),
        "tapioca_syrup": _ingredient(
            "tapioca_syrup",
            water=0.22,
            glucose=0.10,
            fructose=0.02,
            maltodextrin=0.62,
            ash=0.02,
            other_solids=0.02,
            cost=0.0,
            maltodextrin_dp=4.5,
            water_binding=1.0,  # starch fraction traps a modest amount of water
            hydrocolloid=True,
        ),
        "maltodextrin10": _ingredient(
            "maltodextrin10",
            water=0.05,
            maltodextrin=0.93,
            ash=0.02,
            cost=0.0,
            maltodextrin_dp=10.0,
        ),
        "inulin": _ingredient(
            "inulin",
            water=0.05,
            other_solids=0.93,
            ash=0.02,
            cost=0.0,
            effective_mw=5000.0,
            water_binding=3.0,
            maltodextrin_dp=20.0,
        ),
        "glycerol": _ingredient(
            "glycerol",
            water=0.0,
            polyols=0.995,
            cost=0.0,
            polyol_mw=MW_GLYCEROL,
        ),
        "sorbitol": _ingredient(
            "sorbitol",
            water=0.0,
            polyols=0.995,
            cost=0.0,
            polyol_mw=MW_SORBITOL,
        ),
        "erythritol": _ingredient(
            "erythritol",
            water=0.0,
            polyols=0.995,
            cost=0.0,
            polyol_mw=MW_ERYTHRITOL,
        ),
        "guar_gum": _ingredient(
            "guar_gum",
            water=0.10,
            other_solids=0.90,
            cost=0.0,
            effective_mw=80000.0,
            water_binding=12.0,
            hydrocolloid=True,
        ),
        "locust_bean_gum": _ingredient(
            "locust_bean_gum",
            water=0.10,
            other_solids=0.90,
            cost=0.0,
            effective_mw=120000.0,
            water_binding=10.0,
            hydrocolloid=True,
        ),
        "carrageenan": _ingredient(
            "carrageenan",
            water=0.10,
            other_solids=0.90,
            cost=0.0,
            effective_mw=400000.0,
            water_binding=8.0,
            hydrocolloid=True,
        ),
        "xanthan": _ingredient(
            "xanthan",
            water=0.10,
            other_solids=0.90,
            cost=0.0,
            effective_mw=2000000.0,
            water_binding=15.0,
            hydrocolloid=True,
        ),
        "gelatin": _ingredient(
            "gelatin",
            water=0.10,
            other_solids=0.90,
            cost=0.0,
            effective_mw=50000.0,
            water_binding=6.0,
            hydrocolloid=True,
        ),
        "mono_diglycerides": _ingredient(
            "mono_diglycerides",
            water=0.0,
            fat=0.99,
            cost=0.0,
            emulsifier_power=4.0,
        ),
        "lecithin": _ingredient(
            "lecithin",
            water=0.01,
            fat=0.95,
            other_solids=0.04,
            cost=0.0,
            emulsifier_power=3.0,
        ),
        "vanilla_extract": _ingredient(
            "vanilla_extract",
            water=0.55,
            other_solids=0.45,
            cost=0.0,
            effective_mw=200.0,
        ),
        "vanilla_beans": _ingredient(
            "vanilla_beans",
            water=0.10,
            fat=0.05,
            protein=0.05,
            other_solids=0.76,
            ash=0.04,
            cost=0.0,
            effective_mw=600.0,
        ),
        "cocoa_powder": _ingredient(
            "cocoa_powder",
            water=0.03,
            fat=0.22,
            protein=0.20,
            other_solids=0.45,
            ash=0.10,
            cost=0.0,
            effective_mw=500.0,
        ),
        "strawberry_puree": _ingredient(
            "strawberry_puree",
            water=0.90,
            fructose=0.06,
            glucose=0.02,
            sucrose=0.02,
            cost=0.0,
        ),
    }

    def _set_saturated(keys: Sequence[str], bounds: tuple[float, float]) -> None:
        lo, hi = _expand_fraction_bounds(*bounds)
        mid = 0.5 * (lo + hi)
        for name in keys:
            if name not in table:
                continue
            ing = table[name]
            table[name] = ing.clone_with(
                saturated_fat=ing.fat * mid,
                saturated_fat_min=ing.fat * lo,
                saturated_fat_max=ing.fat * hi,
            )

    def _set_lactose_range(keys: Sequence[str], *, tolerance: float = LABEL_PERCENT_EPS) -> None:
        for name in keys:
            if name not in table:
                continue
            ing = table[name]
            lo = max(0.0, ing.lactose * (1 - tolerance))
            hi = ing.lactose * (1 + tolerance)
            table[name] = ing.clone_with(lactose_min=lo, lactose_max=hi)

    dairy_keys = [
        "whole_milk",
        "cream_fat",
        "skim_milk",
        "heavy_cream",
        "whipping_cream",
        "light_cream",
        "cream36",
        "anhydrous_milk_fat",
        "butter",
        "skim_milk_powder",
        "wpc80",
    ]
    _set_saturated(dairy_keys, DAIRY_SAT_FAT_RANGE)
    _set_saturated(["egg_yolk"], EGG_SAT_FAT_RANGE)
    _set_saturated(["vanilla_beans"], PLANT_SAT_FAT_RANGE)
    _set_saturated(["cocoa_powder"], COCOA_SAT_FAT_RANGE)
    _set_saturated(["mono_diglycerides"], (0.40, 0.60))
    _set_saturated(["lecithin"], (0.15, 0.25))
    _set_lactose_range(dairy_keys + ["cream_serum", "milk"])
    _set_lactose_range(["nonfat_milk"])

    def _set_added(name: str, value: float | tuple[float, float]) -> None:
        if name not in table:
            return
        if isinstance(value, tuple):
            lo, hi = value
            mid = 0.5 * (lo + hi)
        else:
            mid = lo = hi = value
        table[name] = table[name].clone_with(
            added_sugars=mid,
            added_sugars_min=lo,
            added_sugars_max=hi,
        )

    def _sugar_fields(name: str, fields: Sequence[str], *, tolerance: float = LABEL_PERCENT_EPS) -> None:
        if name not in table:
            return
        ing = table[name]
        total = sum(getattr(ing, field, 0.0) for field in fields)
        _set_added(name, (max(0.0, total * (1 - tolerance)), total * (1 + tolerance)))

    _sugar_fields("sucrose", ["sucrose"])
    _sugar_fields("dextrose", ["glucose"])
    _sugar_fields("fructose", ["fructose"])
    _sugar_fields("corn_syrup_42", ["glucose", "fructose", "maltodextrin"])
    _sugar_fields("tapioca_syrup", ["glucose", "fructose", "maltodextrin"])
    _sugar_fields("maltodextrin10", ["maltodextrin"])

    return table


def costs_from(table: Mapping[str, Ingredient]) -> Dict[str, float]:
    """Extract a cost dictionary aligned to a given ingredient table."""

    return {k: ing.cost for k, ing in table.items()}


__all__ = ["Ingredient", "default_ingredients", "costs_from"]
