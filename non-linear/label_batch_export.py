"""Batch export of label-driven formulation scenarios to a single HTML page."""

from __future__ import annotations

import html
import sys
from typing import Iterable, Mapping, Sequence

from label_scenarios import (
    LabelScenarioResult,
    solve_ben_jerry_label,
    solve_breyers_vanilla,
    solve_brighams_vanilla,
    solve_haagen_dazs_vanilla,
    solve_jenis_sweet_cream,
    solve_talenti_vanilla,
)
from label_utils import recipe_from_solution


def _scenario_to_row(
    result: LabelScenarioResult,
) -> tuple[dict[str, float | str], Mapping[str, float], Mapping[str, float]]:
    recipe, serving_size_g = recipe_from_solution(
        result.solution,
        result.ingredient_keys,
        result.ingredient_map,
        result.goals,
    )
    formulation = recipe.formulation()
    facts = recipe.nutrition_facts(serving_size_g=serving_size_g)
    fractions = recipe.fractions()
    label = result.label_facts
    metadata = result.metadata or {}

    row: dict[str, float | str] = {
        "name": result.name,
        "pint_mass_g": result.pint_mass_g,
        "label_serving_size_g": label.serving_size_g,
        "label_calories": label.calories,
        "label_total_fat_g": label.total_fat_g,
        "label_total_carbs_g": label.total_carbs_g,
        "label_total_sugars_g": label.total_sugars_g,
        "label_protein_g": label.protein_g,
        "label_saturated_fat_g": label.saturated_fat_g or 0.0,
        "label_trans_fat_g": label.trans_fat_g or 0.0,
        "label_added_sugars_g": label.added_sugars_g or 0.0,
        "label_cholesterol_mg": label.cholesterol_mg or 0.0,
        "label_sodium_mg": label.sodium_mg,
        "target_fat_pct": result.goals.fat_pct,
        "target_solids_pct": result.goals.solids_pct,
        "target_sweetness_pct": result.goals.sweetness_pct,
        "target_freezing_point_C": result.goals.freezing_point_C,
        "target_overrun": result.goals.overrun,
        "target_batch_mass_kg": result.goals.batch_mass,
        "predicted_serving_size_g": serving_size_g,
        "predicted_calories": facts.calories,
        "predicted_total_fat_g": facts.total_fat_g,
        "predicted_saturated_fat_g": facts.saturated_fat_g if facts.saturated_fat_g is not None else 0.0,
        "predicted_total_carbs_g": facts.total_carbs_g,
        "predicted_total_sugars_g": facts.total_sugars_g,
        "predicted_added_sugars_g": facts.added_sugars_g if facts.added_sugars_g is not None else 0.0,
        "predicted_protein_g": facts.protein_g,
        "predicted_trans_fat_g": facts.trans_fat_g or 0.0,
        "predicted_cholesterol_mg": facts.cholesterol_mg or 0.0,
        "predicted_sodium_mg": facts.sodium_mg,
        "predicted_overrun": recipe.overrun,
        "milkfat_pct_mix": formulation.milkfat_pct,
        "snf_pct_mix": formulation.snf_pct,
        "water_pct_mix": formulation.water_pct,
        "protein_pct_mix": formulation.protein_pct,
        "stabilizer_pct_mix": formulation.stabilizer_pct,
        "emulsifier_pct_mix": formulation.emulsifier_pct,
    }

    for key, value in metadata.items():
        row[f"metadata_{key}"] = value

    return row, fractions, formulation.sugars_pct


def _recipe_list_html(fractions: Mapping[str, float]) -> str:
    """Convert fraction map into a bullet list."""

    items = [
        (name, fraction) for name, fraction in sorted(fractions.items(), key=lambda kv: -kv[1]) if fraction > 1e-4
    ]
    if not items:
        return "<em>No ingredients</em>"

    lis = "\n".join(
        f'            <li><strong>{html.escape(name)}</strong><span class="recipe-frac">{fraction*100:.2f}%</span></li>'
        for name, fraction in items
    )
    return f"<ul class=\"fat-ul recipe-list\">\n{lis}\n        </ul>"


def _fat_dl(entries: Sequence[tuple[str | None, str | None]]) -> str:
    filtered = [(label, value) for label, value in entries if value]
    if not filtered:
        return '<div class="placeholder">No data</div>'
    lines: list[str] = []
    for label, value in filtered:
        escaped_value = html.escape(value)
        if label:
            lines.append(f"            <dt>{html.escape(label)}</dt>")
            lines.append(f"            <dd>{escaped_value}</dd>")
        else:
            lines.append(f"            <dd class=\"fat-dl__value\">{escaped_value}</dd>")
    joined = "\n".join(lines)
    return f"<dl class=\"fat-dl\">\n{joined}\n        </dl>"


def _get_numeric(row: Mapping[str, float | str], key: str) -> float | None:
    value = row.get(key)
    if isinstance(value, (int, float)):
        return float(value)
    return None


def _format_percent(value: float | None, digits: int = 2) -> str | None:
    if value is None:
        return None
    return f"{value * 100:.{digits}f}%"


def _format_mass(value: float | None, *, unit: str = "g", digits: int = 0) -> str | None:
    if value is None:
        return None
    fmt = f"{value:.{digits}f}" if digits else f"{value:.0f}"
    return f"{fmt} {unit}"


def _format_temperature(value: float | None) -> str | None:
    if value is None:
        return None
    return f"{value:.2f} C"


def _measure_string(value: float | None, unit: str, digits: int) -> str:
    if value is None:
        return "--"
    fmt = f"{value:.{digits}f}" if digits else f"{value:.0f}"
    suffix = f" {unit}" if unit else ""
    return f"{fmt}{suffix}"


def _format_dual_measure(actual: float | None, predicted: float | None, unit: str, digits: int) -> str | None:
    left = _measure_string(actual, unit, digits)
    right = _measure_string(predicted, unit, digits)
    if left == "--" and right == "--":
        return None
    return f"{left} label / {right} pred"


def _build_name_cell(result: LabelScenarioResult, row: Mapping[str, float | str]) -> str:
    title = f"<div class=\"scenario-name\">{html.escape(result.name)}</div>"
    label_list = result.label_ingredients
    if label_list:
        li_html = "\n".join(f"            <li>{html.escape(item)}</li>" for item in label_list)
        label_html = f"<ol class=\"label-order\">\n{li_html}\n        </ol>"
    else:
        label_html = '<div class="placeholder">No label order</div>'
    return f"{title}\n{label_html}"


def _build_formulation_cell(
    row: Mapping[str, float | str],
    sugars_pct: Mapping[str, float],
) -> str:
    items: list[tuple[str | None, str | None]] = [
        ("Milkfat", _format_percent(_get_numeric(row, "milkfat_pct_mix"))),
        ("SNF", _format_percent(_get_numeric(row, "snf_pct_mix"))),
        ("Water", _format_percent(_get_numeric(row, "water_pct_mix"))),
        ("Protein", _format_percent(_get_numeric(row, "protein_pct_mix"))),
        ("Stabilizer", _format_percent(_get_numeric(row, "stabilizer_pct_mix"), digits=3)),
        ("Emulsifier", _format_percent(_get_numeric(row, "emulsifier_pct_mix"), digits=3)),
    ]
    for sugar, pct in sorted(sugars_pct.items()):
        if pct <= 1e-4:
            continue
        formatted = _format_percent(pct)
        if formatted:
            label = f"Sugar ({sugar.replace('_', ' ')})"
            items.append((label, formatted))
    return _fat_dl(items)


def _build_production_settings_cell(row: Mapping[str, float | str]) -> str:
    items = [
        ("Batch mass", _format_mass(_get_numeric(row, "target_batch_mass_kg"), unit="kg", digits=1)),
        ("Fat target", _format_percent(_get_numeric(row, "target_fat_pct"))),
        ("Solids target", _format_percent(_get_numeric(row, "target_solids_pct"))),
        ("Sweetness target", _format_percent(_get_numeric(row, "target_sweetness_pct"))),
        ("Freezing point", _format_temperature(_get_numeric(row, "target_freezing_point_C"))),
        ("Overrun target", _format_percent(_get_numeric(row, "target_overrun"))),
    ]
    return _fat_dl(items)


def _build_physical_properties_cell(row: Mapping[str, float | str]) -> str:
    items = [
        ("Pint mass", _format_mass(_get_numeric(row, "pint_mass_g"))),
        ("Serving (label)", _format_mass(_get_numeric(row, "label_serving_size_g"))),
        ("Serving (pred)", _format_mass(_get_numeric(row, "predicted_serving_size_g"))),
        ("Overrun (pred)", _format_percent(_get_numeric(row, "predicted_overrun"))),
    ]
    return _fat_dl(items)


def _build_nutrition_cell(row: Mapping[str, float | str]) -> str:
    unit_digits = {"kcal": 0, "g": 1, "mg": 0}
    fields = [
        ("Calories", "label_calories", "predicted_calories", "kcal"),
        ("Total fat", "label_total_fat_g", "predicted_total_fat_g", "g"),
        ("Sat fat", "label_saturated_fat_g", "predicted_saturated_fat_g", "g"),
        ("Trans fat", "label_trans_fat_g", "predicted_trans_fat_g", "g"),
        ("Total carbs", "label_total_carbs_g", "predicted_total_carbs_g", "g"),
        ("Total sugars", "label_total_sugars_g", "predicted_total_sugars_g", "g"),
        ("Added sugars", "label_added_sugars_g", "predicted_added_sugars_g", "g"),
        ("Protein", "label_protein_g", "predicted_protein_g", "g"),
        ("Cholesterol", "label_cholesterol_mg", "predicted_cholesterol_mg", "mg"),
        ("Sodium", "label_sodium_mg", "predicted_sodium_mg", "mg"),
    ]
    items: list[tuple[str | None, str | None]] = []
    for label, actual_key, predicted_key, unit in fields:
        digits = unit_digits.get(unit, 2)
        value = _format_dual_measure(
            _get_numeric(row, actual_key),
            _get_numeric(row, predicted_key),
            unit,
            digits,
        )
        if value:
            items.append((label, value))
    return _fat_dl(items)


def _compose_row_cells(
    result: LabelScenarioResult,
    row: Mapping[str, float | str],
    fractions: Mapping[str, float],
    sugars_pct: Mapping[str, float],
) -> dict[str, str]:
    return {
        "name": _build_name_cell(result, row),
        "formulation": _build_formulation_cell(row, sugars_pct),
        "recipe": _recipe_list_html(fractions),
        "production_settings": _build_production_settings_cell(row),
        "physical_properties": _build_physical_properties_cell(row),
        "nutrition_facts": _build_nutrition_cell(row),
    }


def _collect_table(
    scenarios: Iterable[LabelScenarioResult],
) -> tuple[list[tuple[str, str]], list[dict[str, str]]]:
    contexts: list[tuple[LabelScenarioResult, dict[str, float | str], Mapping[str, float], Mapping[str, float]]] = []
    failures: list[tuple[str, str]] = []

    for result in scenarios:
        base_row, fractions, sugars = _scenario_to_row(result)
        contexts.append((result, base_row, fractions, sugars))
        status = result.solution.solver_status.lower()
        if status != "solve_succeeded":
            failures.append((result.name, result.solution.solver_status))

    if not contexts:
        raise ValueError("No scenarios provided for export")

    if failures:
        for name, status in failures:
            print(f"[ERROR] Scenario '{name}' failed with status '{status}'.", file=sys.stderr)
        raise SystemExit(1)

    columns: list[tuple[str, str]] = [
        ("name", "Name"),
        ("formulation", "Formulation"),
        ("recipe", "Recipe"),
        ("production_settings", "Production settings"),
        ("physical_properties", "Physical properties"),
        ("nutrition_facts", "Nutrition facts (label vs predicted)"),
    ]

    final_rows = [
        _compose_row_cells(result, row, fractions, sugars)
        for result, row, fractions, sugars in contexts
    ]

    return columns, final_rows


def render_html(columns: Sequence[tuple[str, str]], rows: Sequence[Mapping[str, str]]) -> str:
    """Build a standalone HTML table for the exported rows."""

    head_cells = "\n".join(f"            <th>{html.escape(label)}</th>" for _, label in columns)
    body_rows = []
    for row in rows:
        cells = "\n".join(
            f"            <td>{row.get(key, '')}</td>"
            for key, _ in columns
        )
        body_rows.append(f"        <tr>\n{cells}\n        </tr>")
    tbody = "\n".join(body_rows)

    return f"""<!doctype html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <title>Label Scenario Export</title>
    <style>
        body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 1.5rem; }}
        table {{ border-collapse: collapse; width: 100%; font-size: 0.92rem; }}
        th, td {{ border: 1px solid #ccc; padding: 0.3rem 0.5rem; text-align: left; vertical-align: top; }}
        th {{ background: #f5f5f5; position: sticky; top: 0; }}
        tbody tr:nth-child(even) {{ background: #fafafa; }}
        caption {{ font-weight: 600; text-align: left; margin-bottom: 0.5rem; }}
        .scenario-name {{ font-weight: 600; font-size: 1.05rem; margin-bottom: 0.2rem; }}
        .fat-ul {{ list-style: none; margin: 0; padding: 0; }}
        .fat-ul li {{ margin: 0.1rem 0; }}
        .fat-ul li strong {{ display: inline-block; min-width: 8rem; font-weight: 600; }}
        .recipe-list strong {{ min-width: 14rem; }}
        .recipe-frac {{ margin-left: 0.35rem; font-variant-numeric: tabular-nums; }}
        .fat-dl {{ margin: 0; }}
        .fat-dl dt {{ font-weight: 600; margin: 0.15rem 0 0; }}
        .fat-dl dd {{ margin: 0 0 0.2rem 0; }}
        .fat-dl__value {{ margin-left: 0; }}
        .placeholder {{ color: #777; font-style: italic; }}
        .label-order {{ margin: 0.2rem 0 0.4rem 1.25rem; padding-left: 1.25rem; }}
        .label-order li {{ margin: 0.1rem 0; }}
    </style>
</head>
<body>
    <table>
        <caption>Label Scenario Export ({len(rows)} scenarios)</caption>
        <thead>
        <tr>
{head_cells}
        </tr>
        </thead>
        <tbody>
{tbody}
        </tbody>
    </table>
</body>
</html>
"""


def main() -> None:
    scenarios = [
        solve_ben_jerry_label(),
        solve_jenis_sweet_cream(),
        solve_haagen_dazs_vanilla(),
        solve_brighams_vanilla(),
        solve_breyers_vanilla(),
        solve_talenti_vanilla(),
    ]
    columns, rows = _collect_table(scenarios)
    page = render_html(columns, rows)
    sys.stdout.write(page)


if __name__ == "__main__":
    main()
