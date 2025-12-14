"""Scenario-style examples using the refactored library DSL."""

import math
from typing import Any, Optional, Tuple, Sequence, cast

from creamery import (
    Ingredient,
    FormulationGoals,
    FormulationProblem,
    GoalWeights,
    NutritionFacts,
    FormulationSolution,
    Recipe,
    default_ingredients,
)
from label_scenarios import (
    LabelScenarioResult,
    solve_ben_jerry_label,
    solve_jenis_sweet_cream,
    solve_talenti_vanilla,
)
from label_utils import recipe_from_solution


PrecomputedSolution = Tuple[Recipe, float, NutritionFacts]


def print_solution_block(
    solution,
    ingredient_keys,
    ingredients,
    *,
    goals,
    title: str | None = None,
    precomputed: Optional[PrecomputedSolution] = None,
) -> None:
    if title is not None:
        print()
        print(f"=== {title} ===")
        print()
    print("Status:", solution.solver_status)
    if solution.solver_status.lower().startswith("infeasible"):
        print("No feasible solution; skipping output.")
        return

    if precomputed is None:
        recipe, serving_size_g = recipe_from_solution(solution, ingredient_keys, ingredients, goals)
        facts = solution.recipe.nutrition_facts(serving_size_g=serving_size_g)
    else:
        recipe, serving_size_g, facts = precomputed

    print(recipe.__print__())

    formulation = solution.recipe.formulation()
    print(formulation.__print__())
    print(facts.__print__())


def _format_nutrition_value(value: Optional[float], unit: str) -> str:
    if value is None:
        return "   --"
    if unit == "kcal":
        return f"{value:7.0f} {unit}"
    if abs(value) >= 100:
        return f"{value:7.1f} {unit}"
    return f"{value:7.2f} {unit}"


def _print_label_prediction_comparison(label: NutritionFacts, predicted: NutritionFacts) -> None:
    fields = [
        ("Calories", "calories", "kcal"),
        ("Fat", "total_fat_g", "g"),
        ("Saturated fat", "saturated_fat_g", "g"),
        ("Trans fat", "trans_fat_g", "g"),
        ("Cholesterol", "cholesterol_mg", "mg"),
        ("Sodium", "sodium_mg", "mg"),
        ("Total carbs", "total_carbs_g", "g"),
        ("Total sugars", "total_sugars_g", "g"),
        ("Added sugars", "added_sugars_g", "g"),
        ("Protein", "protein_g", "g"),
    ]

    rows = []
    for label_text, attr, unit in fields:
        label_value = getattr(label, attr, None)
        if label_value is None:
            continue
        predicted_value = getattr(predicted, attr, None)
        rows.append(("Label", label_text, label_value, unit))
        rows.append(("Pred", label_text, predicted_value, unit))

    if not rows:
        return

    print("  Nutrition check (label vs predicted):")
    for prefix, label_text, value, unit in rows:
        print(f"    {prefix:>5s} {label_text:<15s}{_format_nutrition_value(value, unit)}")


def balanced_mix_demo() -> None:
    pantry = default_ingredients()
    household_costs = {
        "water": 0.0,
        "heavy_cream": 6.5,  # retail quart ≈ $6.15 → $6.50/kg
        "whole_milk": 0.9,  # gallon ≈ $3.40
        "skim_milk_powder": 8.0,  # pantry tub /1 lb bags
        "sucrose": 1.1,  # granulated sugar 4 lb bag ~ $2.00
        "dextrose": 3.2,  # homebrew/glucose powder bags
        "guar_gum": 18.0,  # specialty baking tubs
    }

    def priced(name: str) -> Ingredient:
        # clone_with keeps the base composition but tags a scenario-specific cost
        return pantry[name].clone_with(cost=household_costs[name])

    active = [
        priced("water"),
        priced("heavy_cream"),
        priced("whole_milk"),
        priced("skim_milk_powder"),
        priced("sucrose"),
        priced("dextrose"),
        priced("guar_gum"),
    ]
    active_map = {ing.name: ing for ing in active}
    goals = FormulationGoals(
        batch_mass=100.0,
        fat_pct=0.12,
        solids_pct=0.38,
        sweetness_pct=0.16,
        freezing_point_C=-3.4,
        overrun=0.30,
    )
    problem = FormulationProblem(ingredients=active, goals=goals)
    problem.add_constraint(problem.props["viscosity_at_serve"] - 0.003, 0.0, math.inf, "viscosity_floor")
    problem.add_constraint(problem.props["meltdown_index"] - 0.75, 0.0, math.inf, "meltdown_floor")
    problem.add_constraint(problem.props["lactose_supersaturation"], -math.inf, 0.75, "lactose_cap")
    problem.add_constraint(problem.props["freezer_load_kj"], -math.inf, 19500.0, "freezer_load_limit")
    problem.add_constraint(problem.props["polymer_solids_pct"], -math.inf, 0.003, "polymer_cap")
    solution = problem.solve()

    print("\nBalanced Premium Mix")
    print()
    print("=== INPUT ===")
    print()
    print("  Ingredients on hand:")
    for ing in active:
        print(f"    - {ing.name}")
    print("  Target macros and process conditions:")
    print(f"    Batch mass        {goals.batch_mass:6.1f} kg")
    print(f"    Milkfat           {goals.fat_pct*100:6.2f} %")
    print(f"    Total solids      {goals.solids_pct*100:6.2f} %")
    print(f"    Sweetness         {goals.sweetness_pct*100:6.2f} % sucrose eq")
    print(f"    Freezing point    {goals.freezing_point_C:6.2f} °C")
    print(f"    Overrun target    {goals.overrun*100:6.1f} %")

    print()
    print("=== OUTPUT ===")
    print()
    print_solution_block(
        solution,
        problem.ingredient_keys,
        active_map,
        goals=goals,
    )


def _print_label_inputs(result: LabelScenarioResult) -> None:
    label = result.label_facts
    goals = result.goals

    print("  Label ingredient order:")
    for item in result.label_ingredients:
        print(f"    - {item}")
    print("  Label nutrition (per serving):")
    print(f"    Serving size     {label.serving_size_g:.0f} g")
    print(f"    Calories         {label.calories:.0f}")
    print(f"    Total fat        {label.total_fat_g:.1f} g")
    if label.saturated_fat_g is not None:
        print(f"    Saturated fat    {label.saturated_fat_g:.1f} g")
    metadata = result.metadata or {}
    if label.trans_fat_g is not None:
        print(f"    Trans fat        {label.trans_fat_g:.1f} g")
    elif "trans_fat_g" in metadata:
        print(f"    Trans fat        {metadata['trans_fat_g']:.1f} g")
    if label.total_carbs_g:
        print(f"    Total carbs      {label.total_carbs_g:.1f} g")
    if label.total_sugars_g:
        print(f"    Total sugars     {label.total_sugars_g:.1f} g")
    if label.added_sugars_g is not None:
        print(f"    Added sugars     {label.added_sugars_g:.1f} g")
    if label.protein_g:
        print(f"    Protein          {label.protein_g:.1f} g")
    if label.cholesterol_mg is not None:
        print(f"    Cholesterol      {label.cholesterol_mg:.0f} mg")
    elif "cholesterol_mg" in metadata:
        print(f"    Cholesterol      {metadata['cholesterol_mg']:.0f} mg")
    if label.sodium_mg:
        print(f"    Sodium           {label.sodium_mg:.0f} mg")

    print("  Derived mix targets:")
    print(f"    Fat              {goals.fat_pct*100:5.2f} %")
    print(f"    Total sugars     {goals.sweetness_pct*100:5.2f} %")
    if label.saturated_fat_g is not None and label.serving_size_g > 0:
        sat_pct = label.saturated_fat_g / label.serving_size_g
        print(f"    Saturated fat    {sat_pct*100:5.2f} % (label)")
    if label.added_sugars_g is not None and label.serving_size_g > 0:
        added_pct = label.added_sugars_g / label.serving_size_g
        print(f"    Added sugars     {added_pct*100:5.2f} % (label)")
    print(f"    Freezing pt      {goals.freezing_point_C:6.2f} °C")
    print(f"    Overrun (label)  {goals.overrun*100:5.1f} %")


def _print_constraint_residuals(solution: FormulationSolution, *, limit: int = 8) -> None:
    diagnostics = solution.diagnostics or {}
    report = diagnostics.get("constraint_report") if isinstance(diagnostics, dict) else None
    if not report:
        print("  (no constraint diagnostics available)")
        return

    report_seq = cast(Sequence[Any], report)
    report_entries: list[dict[str, Any]] = [entry for entry in report_seq if isinstance(entry, dict)]
    if not report_entries:
        print("  (no constraint diagnostics available)")
        return

    def _fmt(value: Any) -> str:
        num = float(value) if isinstance(value, (int, float)) else None
        if num is None:
            return "--"
        if math.isinf(num):
            return " inf" if num > 0 else "-inf"
        return f"{num:8.3f}"

    def _violation(entry: dict[str, Any]) -> float:
        val = entry.get("violation")
        if isinstance(val, (int, float)):
            return float(val)
        return 0.0

    sorted_report = sorted(report_entries, key=lambda entry: abs(_violation(entry)), reverse=True)
    significant = [entry for entry in sorted_report if abs(_violation(entry)) >= 1e-5]
    if not significant:
        print("  (all constraints satisfied within tolerance)")
        return
    print("  Constraint residuals (largest violations):")
    for entry in significant[:limit]:
        note = (entry.get("note") or "(unnamed)")[:24]
        value = entry.get("value")
        lower = entry.get("lower")
        upper = entry.get("upper")
        violation = _violation(entry)
        print(
            f"    {note:24s} value={_fmt(value)} bounds=[{_fmt(lower)}, {_fmt(upper)}] violation={violation:8.4f}"
        )


def _run_label_demo(result: LabelScenarioResult) -> None:
    print(f"\n{result.name}")
    print()
    print("=== INPUT ===")
    print()
    _print_label_inputs(result)
    print()
    print("=== OUTPUT ===")
    print()
    solution = result.solution
    if solution.solver_status.lower().startswith("infeasible"):
        print("  No feasible solution found.")
        _print_constraint_residuals(solution)
        return

    recipe, serving_size_g = recipe_from_solution(
        solution,
        result.ingredient_keys,
        result.ingredient_map,
        goals=result.goals,
    )
    predicted_facts = solution.recipe.nutrition_facts(serving_size_g=serving_size_g)
    print_solution_block(
        solution,
        result.ingredient_keys,
        result.ingredient_map,
        goals=result.goals,
        precomputed=(recipe, serving_size_g, predicted_facts),
    )
    _print_label_prediction_comparison(result.label_facts, predicted_facts)


def ben_jerry_label_demo() -> None:
    _run_label_demo(solve_ben_jerry_label())


def jenis_sweet_cream_demo() -> None:
    _run_label_demo(solve_jenis_sweet_cream())


def talenti_vanilla_demo() -> None:
    _run_label_demo(solve_talenti_vanilla())


if __name__ == "__main__":
    balanced_mix_demo()
    ben_jerry_label_demo()
    jenis_sweet_cream_demo()
    talenti_vanilla_demo()
