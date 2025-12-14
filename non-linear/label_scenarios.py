"""Reusable label-driven formulation scenarios."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Mapping, Sequence
import math

import casadi as ca

from creamery import (
    Ingredient,
    LabelGroup,
    NutritionFacts,
    FormulationGoals,
    FormulationProblem,
    FormulationSolution,
    GoalWeights,
    apply_group_bounds,
    apply_label_order,
    default_ingredients,
    goals_from_label,
)
from creamery.constants import LABEL_PERCENT_EPS


_EPS = LABEL_PERCENT_EPS


@dataclass(frozen=True)
class LabelScenarioResult:
    """Container for solved label scenarios."""

    name: str
    label_ingredients: Sequence[str]
    label_facts: NutritionFacts
    goals: FormulationGoals
    solution: FormulationSolution
    ingredient_keys: Sequence[str]
    ingredient_map: Mapping[str, Ingredient]
    pint_mass_g: float
    metadata: Mapping[str, float] | None = None


def _fraction_bounds(value: float) -> tuple[float, float]:
    if value <= 0:
        return 0.0, 0.0
    lo = max(0.0, value * (1 - _EPS))
    hi = min(1.0, value * (1 + _EPS))
    return lo, hi


def _scaled_bounds(value: float, *, floor: float = 0.0) -> tuple[float, float]:
    if value <= 0:
        return floor, floor
    lo = max(floor, value * (1 - _EPS))
    hi = value * (1 + _EPS)
    return lo, hi


def _range_with_eps(lo: float, hi: float) -> tuple[float, float]:
    return (max(0.0, lo * (1 - _EPS)), min(1.0, hi * (1 + _EPS)))


def _mass_from_serving(value_g: float, facts: NutritionFacts, goals: FormulationGoals) -> float:
    if facts.serving_size_g <= 0:
        return 0.0
    ratio = value_g / facts.serving_size_g
    return ratio * goals.batch_mass


def _fraction_from_label(value_g: float | None, facts: NutritionFacts) -> float | None:
    if value_g is None or facts.serving_size_g <= 0:
        return None
    return value_g / facts.serving_size_g


def _chol_mg_per_kg(value_mg: float, facts: NutritionFacts) -> float:
    serving_kg = facts.serving_size_g / 1000.0
    if serving_kg <= 0:
        return 0.0
    return value_mg / serving_kg


def _presence_floor(goals: FormulationGoals) -> float:
    return goals.batch_mass * _EPS * 1e-3


def _order_epsilon(goals: FormulationGoals) -> float:
    return max(1e-6, goals.batch_mass * _EPS * 0.1)


def _add_fraction_preference(
    problem: FormulationProblem,
    numerator_key: str,
    denominator_keys: Sequence[str],
    preferred: tuple[float, float],
    weight: float = 250.0,
) -> None:
    total = problem.sum_of(denominator_keys)
    frac = problem.expr(numerator_key) / ca.fmax(1e-6, total)
    lo, hi = preferred
    penalty = ca.fmax(0, lo - frac) ** 2 + ca.fmax(0, frac - hi) ** 2
    problem.add_soft_penalty(penalty, weight)


def _apply_label_macro_constraints(problem: FormulationProblem, facts: NutritionFacts) -> None:
    goals = problem.goals

    def _bind(expr, target_pct: float, note: str) -> None:
        lo, hi = _fraction_bounds(target_pct)
        problem.add_constraint(expr, lo, hi, note)

    _bind(problem.fat_pct, goals.fat_pct, "label_fat_pct")

    sugars_pct = _fraction_from_label(facts.total_sugars_g, facts)
    if sugars_pct is not None:
        _bind(problem.props["total_sugars_pct"], sugars_pct, "label_total_sugars_pct")


def _apply_label_nutrient_constraints(problem: FormulationProblem, facts: NutritionFacts) -> None:
    """Match optional label nutrients (trans fat, sat fat, added sugars, cholesterol)."""

    goals = problem.goals
    if facts.serving_size_g <= 0:
        return

    if facts.trans_fat_g is not None:
        if facts.trans_fat_g <= 0.1:
            upper_mass = _mass_from_serving(0.5, facts, goals)
            upper = upper_mass * (1 + _EPS)
            problem.add_constraint(problem.props["trans_fat"], 0.0, upper, "label_trans_fat_max")
        else:
            target = _mass_from_serving(facts.trans_fat_g, facts, goals)
            lo, hi = _scaled_bounds(target)
            problem.add_constraint(problem.props["trans_fat"], lo, hi, "label_trans_fat")

    if facts.cholesterol_mg is not None:
        target = _chol_mg_per_kg(facts.cholesterol_mg, facts)
        if facts.cholesterol_mg <= 5.0:
            upper = _chol_mg_per_kg(5.0, facts) * (1 + _EPS)
            problem.add_constraint(problem.props["cholesterol_mg_per_kg"], 0.0, upper, "label_cholesterol_cap")
        else:
            lo, hi = _scaled_bounds(target)
            problem.add_constraint(problem.props["cholesterol_mg_per_kg"], lo, hi, "label_cholesterol")

    if facts.saturated_fat_g is not None:
        target = _mass_from_serving(facts.saturated_fat_g, facts, goals)
        lo, hi = _scaled_bounds(target)
        problem.add_constraint(problem.props["saturated_fat"], lo, hi, "label_saturated_fat")

    if facts.added_sugars_g is not None:
        target = _mass_from_serving(facts.added_sugars_g, facts, goals)
        lo, hi = _scaled_bounds(target)
        problem.add_constraint(problem.props["added_sugars"], lo, hi, "label_added_sugars")


def solve_ben_jerry_label() -> LabelScenarioResult:
    """Solve the Ben & Jerry's Vanilla label reconstruction problem."""

    base = default_ingredients()
    active = [
        base["cream_fat"],
        base["cream_serum"],
        base["skim_milk"],
        base["sucrose"].clone(name="liquid_sugar_sucrose"),
        base["water"].clone(name="liquid_sugar_water"),
        base["water"],
        base["egg_yolk"],
        base["sucrose"],
        base["guar_gum"],
        base["vanilla_extract"],
        base["vanilla_beans"],
        base["carrageenan"],
    ]

    label = NutritionFacts(
        serving_size_g=143.0,
        calories=330.0,
        total_fat_g=21.3,
        total_carbs_g=28.7,
        total_sugars_g=28.3,
        protein_g=5.7,
        sodium_mg=67.0,
        saturated_fat_g=14.0,
        added_sugars_g=21.0,
    )
    pint_mass_g = 430.0

    groups = [
        LabelGroup("cream", ["cream_fat", "cream_serum"], fraction_bounds={"cream_fat": _range_with_eps(0.18, 0.50)}),
        LabelGroup("skim_milk", ["skim_milk"]),
        LabelGroup(
            "liquid_sugar",
            ["liquid_sugar_sucrose", "liquid_sugar_water"],
            enforce_internal_order=True,
        ),
        LabelGroup("water", ["water"]),
        LabelGroup("egg_yolk", ["egg_yolk"]),
        LabelGroup("sucrose", ["sucrose"]),
        LabelGroup("guar_gum", ["guar_gum"]),
        LabelGroup("vanilla_extract", ["vanilla_extract"]),
        LabelGroup("vanilla_beans", ["vanilla_beans"]),
        LabelGroup("carrageenan", ["carrageenan"]),
    ]

    goals = goals_from_label(label, pint_mass_g)
    problem = FormulationProblem(
        ingredients=active,
        goals=goals,
        weights=GoalWeights(fat=2500.0, solids=1800.0, sweetness=1400.0, freezing_point=50.0, overrun=50.0),
    )
    apply_group_bounds(problem, groups)
    apply_label_order(problem, groups, epsilon=_order_epsilon(goals))

    presence_floor = _presence_floor(goals)
    for key in [
        "cream_fat",
        "cream_serum",
        "skim_milk",
        "liquid_sugar_sucrose",
        "liquid_sugar_water",
        "water",
        "egg_yolk",
        "sucrose",
        "guar_gum",
        "vanilla_extract",
        "vanilla_beans",
        "carrageenan",
    ]:
        problem.add_constraint(problem.expr(key), presence_floor, math.inf, f"{key}_presence")
    _apply_label_macro_constraints(problem, label)
    _apply_label_nutrient_constraints(problem, label)
    _add_fraction_preference(problem, "cream_fat", ["cream_fat", "cream_serum"], (0.18, 0.40))
    solution = problem.solve()
    label_ingredients = [
        "cream",
        "skim milk",
        "liquid sugar (sucrose, water)",
        "water",
        "egg yolks",
        "sugar",
        "guar gum",
        "vanilla extract",
        "vanilla beans",
        "carrageenan",
    ]

    return LabelScenarioResult(
        name="Ben & Jerry's Vanilla",
        label_ingredients=label_ingredients,
        label_facts=label,
        goals=goals,
        solution=solution,
        ingredient_keys=problem.ingredient_keys,
        ingredient_map=problem.ingredients,
        pint_mass_g=pint_mass_g,
    )


def solve_jenis_sweet_cream() -> LabelScenarioResult:
    """Solve the Jeni's Sweet Cream label reconstruction problem."""

    base = default_ingredients()
    milk = base["whole_milk"].clone_with(name="milk")
    cream_fat = base["cream_fat"]
    cream_serum = base["cream_serum"]
    cane_sugar = base["sucrose"].clone_with(name="cane_sugar")
    nonfat_milk = base["skim_milk"].clone_with(name="nonfat_milk")
    tapioca_syrup = base["tapioca_syrup"]

    active = [milk, cream_fat, cream_serum, cane_sugar, nonfat_milk, tapioca_syrup]
    label = NutritionFacts(
        serving_size_g=124.0,
        calories=316.0,
        total_fat_g=20.0,
        total_carbs_g=28.0,
        total_sugars_g=23.0,
        protein_g=6.0,
        sodium_mg=75.0,
        saturated_fat_g=11.0,
        added_sugars_g=16.0,
        trans_fat_g=1.0,
        cholesterol_mg=55.0,
    )
    pint_mass_g = label.serving_size_g * 3.0

    groups = [
        LabelGroup("milk", ["milk"]),
        LabelGroup("cream", ["cream_fat", "cream_serum"], fraction_bounds={"cream_fat": _range_with_eps(0.18, 0.50)}),
        LabelGroup("cane_sugar", ["cane_sugar"]),
        LabelGroup("nonfat_milk", ["nonfat_milk"]),
        LabelGroup("tapioca_syrup", ["tapioca_syrup"], enforce_internal_order=True),
    ]

    goals = goals_from_label(label, pint_mass_g)
    problem = FormulationProblem(
        ingredients=active,
        goals=goals,
        weights=GoalWeights(fat=2200.0, solids=1600.0, sweetness=1800.0, freezing_point=50.0, overrun=40.0),
    )
    apply_group_bounds(problem, groups)
    apply_label_order(problem, groups, epsilon=_order_epsilon(goals))

    presence_floor = _presence_floor(goals)
    for key in ["milk", "cream_fat", "cream_serum", "cane_sugar", "nonfat_milk", "tapioca_syrup"]:
        problem.add_constraint(problem.expr(key), presence_floor, math.inf, f"{key}_presence")

    _apply_label_macro_constraints(problem, label)
    _apply_label_nutrient_constraints(problem, label)
    _add_fraction_preference(problem, "cream_fat", ["cream_fat", "cream_serum"], (0.18, 0.40))
    solution = problem.solve()
    label_ingredients = [
        "milk",
        "cream",
        "cane sugar",
        "nonfat milk",
        "tapioca syrup",
    ]

    return LabelScenarioResult(
        name="Jeni's Sweet Cream",
        label_ingredients=label_ingredients,
        label_facts=label,
        goals=goals,
        solution=solution,
        ingredient_keys=problem.ingredient_keys,
        ingredient_map=problem.ingredients,
        pint_mass_g=pint_mass_g,
    )


def solve_haagen_dazs_vanilla() -> LabelScenarioResult:
    """Solve the Haagen-Dazs Vanilla label reconstruction problem."""

    base = default_ingredients()
    ingredients = [
        base["cream_fat"],
        base["cream_serum"],
        base["skim_milk"],
        base["sucrose"].clone_with(name="cane_sugar"),
        base["egg_yolk"],
        base["vanilla_extract"],
    ]

    label = NutritionFacts(
        serving_size_g=129.0,
        calories=320.0,
        total_fat_g=21.0,
        total_carbs_g=26.0,
        total_sugars_g=25.0,
        protein_g=6.0,
        sodium_mg=75.0,
        saturated_fat_g=13.0,
        added_sugars_g=18.0,
        trans_fat_g=1.0,
        cholesterol_mg=95.0,
    )
    pint_mass_g = label.serving_size_g * 3.0

    groups = [
        LabelGroup("cream", ["cream_fat", "cream_serum"], fraction_bounds={"cream_fat": _range_with_eps(0.18, 0.50)}),
        LabelGroup("skim_milk", ["skim_milk"]),
        LabelGroup("cane_sugar", ["cane_sugar"]),
        LabelGroup("egg_yolk", ["egg_yolk"]),
        LabelGroup("vanilla_extract", ["vanilla_extract"]),
    ]

    goals = goals_from_label(label, pint_mass_g)
    problem = FormulationProblem(
        ingredients=ingredients,
        goals=goals,
        weights=GoalWeights(fat=2600.0, solids=1900.0, sweetness=1600.0, freezing_point=45.0, overrun=35.0),
    )
    apply_group_bounds(problem, groups)
    apply_label_order(problem, groups, epsilon=_order_epsilon(goals))

    presence_floor = _presence_floor(goals)
    for key in ["cream_fat", "cream_serum", "skim_milk", "cane_sugar", "egg_yolk", "vanilla_extract"]:
        problem.add_constraint(problem.expr(key), presence_floor, math.inf, f"{key}_presence")

    _apply_label_macro_constraints(problem, label)
    _apply_label_nutrient_constraints(problem, label)
    _add_fraction_preference(problem, "cream_fat", ["cream_fat", "cream_serum"], (0.18, 0.40))
    solution = problem.solve()
    label_ingredients = ["cream", "skim milk", "cane sugar", "egg yolks", "vanilla extract"]

    return LabelScenarioResult(
        name="Haagen-Dazs Vanilla",
        label_ingredients=label_ingredients,
        label_facts=label,
        goals=goals,
        solution=solution,
        ingredient_keys=problem.ingredient_keys,
        ingredient_map=problem.ingredients,
        pint_mass_g=pint_mass_g,
    )


def solve_brighams_vanilla() -> LabelScenarioResult:
    """Solve the Brigham's Vanilla label reconstruction problem."""

    base = default_ingredients()
    cream_fat = base["cream_fat"]
    cream_serum = base["cream_serum"]
    milk = base["whole_milk"].clone_with(name="milk")
    sugar = base["sucrose"].clone_with(name="sugar")
    vanilla_extract = base["vanilla_extract"]
    guar_gum = base["guar_gum"]
    carrageenan = base["carrageenan"]
    mono_diglycerides = base["mono_diglycerides"]
    ps80 = Ingredient(name="ps80", water=0.0, other_solids=1.0, emulsifier_power=5.0)
    potassium_phosphate = Ingredient(name="potassium_phosphate", water=0.0, ash=1.0)
    cellulose_gum = base["xanthan"].clone_with(name="cellulose_gum")
    salt = Ingredient(name="salt", water=0.0, ash=1.0)

    active = [
        cream_fat,
        cream_serum,
        milk,
        sugar,
        vanilla_extract,
        guar_gum,
        salt,
        mono_diglycerides,
        ps80,
        carrageenan,
        potassium_phosphate,
        cellulose_gum,
    ]

    label = NutritionFacts(
        serving_size_g=111.0,
        calories=260.0,
        total_fat_g=17.0,
        total_carbs_g=25.0,
        total_sugars_g=23.0,
        protein_g=4.0,
        sodium_mg=95.0,
        saturated_fat_g=10.0,
        added_sugars_g=17.0,
        trans_fat_g=0.5,
        cholesterol_mg=65.0,
    )
    pint_mass_g = label.serving_size_g * 3.0

    groups = [
        LabelGroup("cream", ["cream_fat", "cream_serum"], fraction_bounds={"cream_fat": _range_with_eps(0.18, 0.50)}),
        LabelGroup("milk", ["milk"]),
        LabelGroup("sugar", ["sugar"]),
        LabelGroup("vanilla_extract", ["vanilla_extract"]),
        LabelGroup("guar_gum", ["guar_gum"]),
        LabelGroup("salt", ["salt"]),
        LabelGroup("mono_diglycerides", ["mono_diglycerides"]),
        LabelGroup("ps80", ["ps80"]),
        LabelGroup("carrageenan", ["carrageenan"]),
        LabelGroup("potassium_phosphate", ["potassium_phosphate"]),
        LabelGroup("cellulose_gum", ["cellulose_gum"]),
    ]

    goals = goals_from_label(label, pint_mass_g)
    problem = FormulationProblem(
        ingredients=active,
        goals=goals,
        weights=GoalWeights(fat=2300.0, solids=1700.0, sweetness=1600.0, freezing_point=50.0, overrun=60.0),
    )
    apply_group_bounds(problem, groups)
    apply_label_order(problem, groups, epsilon=_order_epsilon(goals))

    presence_floor = _presence_floor(goals)
    for key in [
        "cream_fat",
        "cream_serum",
        "milk",
        "sugar",
        "vanilla_extract",
        "guar_gum",
        "salt",
        "mono_diglycerides",
        "ps80",
        "carrageenan",
        "potassium_phosphate",
        "cellulose_gum",
    ]:
        problem.add_constraint(problem.expr(key), presence_floor, math.inf, f"{key}_presence")

    _apply_label_macro_constraints(problem, label)
    _apply_label_nutrient_constraints(problem, label)
    _add_fraction_preference(problem, "cream_fat", ["cream_fat", "cream_serum"], (0.18, 0.40))
    solution = problem.solve()
    label_ingredients = [
        "cream",
        "milk",
        "sugar",
        "vanilla extract",
        "guar gum",
        "salt",
        "mono & diglycerides",
        "ps80",
        "carrageenan",
        "potassium phosphate",
        "cellulose gum",
    ]

    return LabelScenarioResult(
        name="Brigham's Vanilla",
        label_ingredients=label_ingredients,
        label_facts=label,
        goals=goals,
        solution=solution,
        ingredient_keys=problem.ingredient_keys,
        ingredient_map=problem.ingredients,
        pint_mass_g=pint_mass_g,
    )


def solve_breyers_vanilla() -> LabelScenarioResult:
    """Solve the Breyers Vanilla label reconstruction problem."""

    base = default_ingredients()
    milk = base["whole_milk"].clone_with(name="milk")
    cream_fat = base["cream_fat"]
    cream_serum = base["cream_serum"]
    sugar = base["sucrose"].clone_with(name="sugar")
    skim_milk = base["skim_milk"].clone_with(name="skim_milk")
    tara_gum = Ingredient(name="tara_gum", water=0.12, other_solids=0.88, hydrocolloid=True)
    natural_flavor = base["vanilla_extract"].clone_with(name="natural_flavor")

    active = [milk, cream_fat, cream_serum, sugar, skim_milk, tara_gum, natural_flavor]

    label = NutritionFacts(
        serving_size_g=88.0,
        calories=170.0,
        total_fat_g=9.0,
        total_carbs_g=19.0,
        total_sugars_g=19.0,
        protein_g=3.0,
        sodium_mg=50.0,
        saturated_fat_g=6.0,
        added_sugars_g=14.0,
        trans_fat_g=0.0,
        cholesterol_mg=25.0,
    )
    pint_mass_g = label.serving_size_g * 3.0

    groups = [
        LabelGroup("milk", ["milk"]),
        LabelGroup("cream", ["cream_fat", "cream_serum"], fraction_bounds={"cream_fat": _range_with_eps(0.18, 0.50)}),
        LabelGroup("sugar", ["sugar"]),
        LabelGroup("skim_milk", ["skim_milk"]),
        LabelGroup("tara_gum", ["tara_gum"]),
        LabelGroup("natural_flavor", ["natural_flavor"]),
    ]

    goals = goals_from_label(label, pint_mass_g)
    problem = FormulationProblem(
        ingredients=active,
        goals=goals,
        weights=GoalWeights(fat=2000.0, solids=1600.0, sweetness=1500.0, freezing_point=55.0, overrun=75.0),
    )
    apply_group_bounds(problem, groups)
    apply_label_order(problem, groups, epsilon=_order_epsilon(goals))

    presence_floor = _presence_floor(goals)
    for key in ["milk", "cream_fat", "cream_serum", "sugar", "skim_milk", "tara_gum", "natural_flavor"]:
        problem.add_constraint(problem.expr(key), presence_floor, math.inf, f"{key}_presence")

    _apply_label_macro_constraints(problem, label)
    _apply_label_nutrient_constraints(problem, label)
    _add_fraction_preference(problem, "cream_fat", ["cream_fat", "cream_serum"], (0.18, 0.40))
    solution = problem.solve()
    label_ingredients = ["milk", "cream", "sugar", "skim milk", "tara gum", "natural flavor"]

    return LabelScenarioResult(
        name="Breyers Vanilla",
        label_ingredients=label_ingredients,
        label_facts=label,
        goals=goals,
        solution=solution,
        ingredient_keys=problem.ingredient_keys,
        ingredient_map=problem.ingredients,
        pint_mass_g=pint_mass_g,
    )


def solve_talenti_vanilla() -> LabelScenarioResult:
    """Solve the Talenti Vanilla Bean gelato reconstruction problem."""

    base = default_ingredients()
    milk = base["whole_milk"].clone_with(name="milk")
    cream_fat = base["cream_fat"]
    cream_serum = base["cream_serum"]
    sugar = base["sucrose"].clone_with(name="sugar")
    dextrose = base["dextrose"]
    vanilla_extract = base["vanilla_extract"]
    sunflower_lecithin = base["lecithin"].clone_with(name="sunflower_lecithin")
    carob_gum = base["locust_bean_gum"].clone_with(name="carob_bean_gum")
    guar_gum = base["guar_gum"]
    natural_flavor = base["vanilla_extract"].clone_with(name="natural_flavor", water=0.60, other_solids=0.40)
    lemon_peel = Ingredient(name="lemon_peel", water=0.20, other_solids=0.70, ash=0.10, effective_mw=800.0)

    active = [
        milk,
        sugar,
        cream_fat,
        cream_serum,
        dextrose,
        vanilla_extract,
        sunflower_lecithin,
        carob_gum,
        guar_gum,
        natural_flavor,
        lemon_peel,
    ]

    label = NutritionFacts(
        serving_size_g=128.0,
        calories=260.0,
        total_fat_g=13.0,
        total_carbs_g=31.0,
        total_sugars_g=30.0,
        protein_g=5.0,
        sodium_mg=70.0,
        saturated_fat_g=8.0,
        added_sugars_g=22.0,
        trans_fat_g=0.0,
        cholesterol_mg=45.0,
    )
    pint_mass_g = label.serving_size_g * 3.0

    groups = [
        LabelGroup("milk", ["milk"]),
        LabelGroup("sugar", ["sugar"]),
        LabelGroup("cream", ["cream_fat", "cream_serum"], fraction_bounds={"cream_fat": _range_with_eps(0.18, 0.50)}),
        LabelGroup("dextrose", ["dextrose"]),
        LabelGroup("vanilla_extract", ["vanilla_extract"]),
        LabelGroup("sunflower_lecithin", ["sunflower_lecithin"]),
        LabelGroup("carob_bean_gum", ["carob_bean_gum"]),
        LabelGroup("guar_gum", ["guar_gum"]),
        LabelGroup("natural_flavor", ["natural_flavor"]),
        LabelGroup("lemon_peel", ["lemon_peel"]),
    ]

    goals = goals_from_label(label, pint_mass_g)
    problem = FormulationProblem(
        ingredients=active,
        goals=goals,
        weights=GoalWeights(fat=2300.0, solids=1700.0, sweetness=1700.0, freezing_point=55.0, overrun=60.0),
    )
    apply_group_bounds(problem, groups)
    apply_label_order(problem, groups, epsilon=_order_epsilon(goals))

    presence_floor = _presence_floor(goals)
    for key in [
        "milk",
        "sugar",
        "cream_fat",
        "cream_serum",
        "dextrose",
        "vanilla_extract",
        "sunflower_lecithin",
        "carob_bean_gum",
        "guar_gum",
        "natural_flavor",
        "lemon_peel",
    ]:
        problem.add_constraint(problem.expr(key), presence_floor, math.inf, f"{key}_presence")

    _apply_label_macro_constraints(problem, label)
    _apply_label_nutrient_constraints(problem, label)
    _add_fraction_preference(problem, "cream_fat", ["cream_fat", "cream_serum"], (0.18, 0.40))
    solution = problem.solve()
    label_ingredients = [
        "milk",
        "sugar",
        "cream",
        "dextrose",
        "vanilla extract",
        "sunflower lecithin",
        "carob bean gum",
        "guar gum",
        "natural flavor",
        "lemon peel",
    ]

    return LabelScenarioResult(
        name="Talenti Vanilla Bean",
        label_ingredients=label_ingredients,
        label_facts=label,
        goals=goals,
        solution=solution,
        ingredient_keys=problem.ingredient_keys,
        ingredient_map=problem.ingredients,
        pint_mass_g=pint_mass_g,
    )


__all__ = [
    "LabelScenarioResult",
    "solve_ben_jerry_label",
    "solve_jenis_sweet_cream",
    "solve_haagen_dazs_vanilla",
    "solve_brighams_vanilla",
    "solve_breyers_vanilla",
    "solve_talenti_vanilla",
]
