"""Lightweight typed containers for recipes, formulations, and nutrition facts."""

from __future__ import annotations

from collections import OrderedDict
from dataclasses import dataclass, replace
from typing import Dict, Sequence, Tuple

from .ingredients import Ingredient
from .chemistry import component_sums, build_properties, sweetness_eq


# --------------------------- Data containers --------------------------- #


@dataclass(frozen=True)
class NutritionFacts:
    """FDA-style per-serving facts with optional mix percentages."""

    serving_size_g: float
    calories: float
    total_fat_g: float
    total_carbs_g: float
    total_sugars_g: float
    protein_g: float
    sodium_mg: float = 0.0
    saturated_fat_g: float | None = None
    added_sugars_g: float | None = None
    trans_fat_g: float | None = None
    cholesterol_mg: float | None = None
    fat_pct: float | None = None
    carbs_pct: float | None = None
    sugars_pct: float | None = None
    protein_pct: float | None = None
    trans_fat_pct: float | None = None
    saturated_fat_pct: float | None = None
    added_sugars_pct: float | None = None
    cholesterol_mg_per_kg: float | None = None

    def __print__(self) -> str:
        ss = self.serving_size_g
        def macro(label: str, grams: float, pct: float | None) -> str:
            if pct is None:
                return f"  {label:14s}{grams:6.2f} g"
            return f"  {label:14s}{grams:6.2f} g ({pct*100:5.2f}% mix)"

        sugar_line = macro("Total sugars", self.total_sugars_g, self.sugars_pct)
        lines = [
            f"Nutrition facts (per {ss:.0f} g):",
            f"  Calories       {self.calories:6.1f} kcal",
            macro("Fat", self.total_fat_g, self.fat_pct),
        ]
        if self.saturated_fat_g is not None:
            lines.append(f"  Saturated fat  {self.saturated_fat_g:6.2f} g")
        if self.trans_fat_g is not None:
            lines.append(macro("Trans fat", self.trans_fat_g, self.trans_fat_pct))
        if self.cholesterol_mg is not None:
            chol_line = f"  Cholesterol    {self.cholesterol_mg:6.1f} mg"
            if self.cholesterol_mg_per_kg is not None:
                chol_line += f" ({self.cholesterol_mg_per_kg:6.1f} mg/kg mix)"
            lines.append(chol_line)
        if self.sodium_mg:
            lines.append(f"  Sodium         {self.sodium_mg:6.1f} mg")
        carb_lines = [
            macro("Total carbs", self.total_carbs_g, self.carbs_pct),
            "    " + sugar_line.lstrip(),
        ]
        if self.added_sugars_g is not None:
            carb_lines.append(f"    Added sugars   {self.added_sugars_g:6.2f} g")
        carb_lines.append(macro("Protein", self.protein_g, self.protein_pct))
        lines.extend(carb_lines)
        return "\n".join(lines)

    def __str__(self) -> str:  # pragma: no cover - convenience for print()
        return self.__print__()


@dataclass(frozen=True)
class Formulation:
    """Aggregate composition expressed as batch mass fractions."""

    milkfat_pct: float
    snf_pct: float
    water_pct: float
    sugars_pct: Dict[str, float]
    stabilizer_pct: float
    emulsifier_pct: float
    protein_pct: float

    def __print__(self) -> str:
        lines = [
            "Formulation:",
            f"  Milkfat        {self.milkfat_pct*100:5.2f} %",
            f"  SNF            {self.snf_pct*100:5.2f} %",
            f"  Water          {self.water_pct*100:5.2f} %",
            f"  Protein        {self.protein_pct*100:5.2f} %",
            f"  Stabilizer     {self.stabilizer_pct*100:5.3f} %",
            f"  Emulsifier     {self.emulsifier_pct*100:5.3f} %",
        ]
        sugar_lines = [
            f"    {k:10s} {v*100:5.2f} %"
            for k, v in sorted(self.sugars_pct.items())
            if v > 1e-6
        ]
        if sugar_lines:
            lines.append("  Sugars:")
            lines.extend(sugar_lines)
        return "\n".join(lines)

    def __str__(self) -> str:  # pragma: no cover - convenience for print()
        return self.__print__()

    @property
    def msnf_pct(self) -> float:
        """Backward-compatible alias for SNF percentage."""

        return object.__getattribute__(self, "snf_pct")


@dataclass(frozen=True)
class ProductionSettings:
    """Instantaneous record of mix conditions plus derived properties."""

    serve_temp_C: float
    draw_temp_C: float
    shear_rate_s: float
    overrun_cap: float | None
    metrics: Dict[str, float]


@dataclass(frozen=True)
class Recipe:
    """Concrete ingredient masses (kg) and process metadata."""

    components: Sequence[Tuple[Ingredient, float]]
    overrun: float = 0.0
    notes: Sequence[str] | None = None
    mix_snapshot: ProductionSettings | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "components", tuple(self.components))
        if self.notes is not None:
            object.__setattr__(self, "notes", tuple(self.notes))
        if self.overrun < 0:
            raise ValueError("Overrun cannot be negative.")
        for ing, weight in self.components:
            if weight < 0:
                raise ValueError(f"Ingredient '{ing.name}' weight cannot be negative.")

    @classmethod
    def from_weights(
        cls,
        ingredients: Sequence[Ingredient],
        weights: Sequence[float],
        *,
        overrun: float = 0.0,
    ) -> "Recipe":
        if len(ingredients) != len(weights):
            raise ValueError("Ingredient and weight sequences must have equal length.")
        entries = []
        for ing, weight in zip(ingredients, weights):
            mass = float(weight)
            if mass <= 0:
                continue
            entries.append((ing, mass))
        return cls(entries, overrun=overrun)

    def with_overrun(self, overrun: float) -> "Recipe":
        if overrun < 0:
            raise ValueError("Overrun cannot be negative.")
        return replace(self, overrun=overrun)

    def with_notes(self, notes: Sequence[str] | None) -> "Recipe":
        return replace(self, notes=tuple(notes) if notes is not None else None)

    def with_mix_snapshot(self, snapshot: ProductionSettings | None) -> "Recipe":
        return replace(self, mix_snapshot=snapshot)

    @property
    def batch_mass_kg(self) -> float:
        return sum(weight for _, weight in self.components)

    def fractions(self) -> Dict[str, float]:
        total = self.batch_mass_kg
        if total <= 0:
            return {}
        fractions: Dict[str, float] = {}
        for ing, weight in self.components:
            if weight <= 0:
                continue
            fractions[ing.name] = fractions.get(ing.name, 0.0) + weight / total
        return fractions

    def __print__(self) -> str:
        total = self.batch_mass_kg
        lines = ["Recipe:", "  Ingredients:"]
        ordered = sorted(self.components, key=lambda kv: -kv[1])
        for ing, weight in ordered:
            if weight <= 1e-4:
                continue
            pct = (weight / total * 100) if total > 0 else 0.0
            lines.append(f"    {ing.name:20s} {pct:6.2f} %")
        if self.mix_snapshot is not None:
            snapshot = self.mix_snapshot
            metrics = snapshot.metrics
            total_mass = metrics.get("total_mass", total)
            lines.append("  Production settings:")
            lines.append(f"    Serve temp       {snapshot.serve_temp_C:6.2f} °C")
            lines.append(f"    Draw temp        {snapshot.draw_temp_C:6.2f} °C")
            lines.append(f"    Shear rate       {snapshot.shear_rate_s:6.1f} s^-1")
            lines.append(f"    Overrun          {metrics.get('overrun_actual', self.overrun)*100:6.2f} %")
            if snapshot.overrun_cap is not None:
                lines.append(f"    Overrun cap      {snapshot.overrun_cap*100:6.2f} %")
            lines.append("  Physical properties:")
            def pct(value: float) -> float:
                return 0.0 if total_mass <= 0 else value / total_mass * 100.0
            prop_specs = [
                ("Water", metrics.get("water"), lambda v: f"{pct(v):6.2f} %"),
                ("Bound water", metrics.get("bound_water"), lambda v: f"{pct(v):6.2f} %"),
                ("Fat", metrics.get("fat"), lambda v: f"{pct(v):6.2f} %"),
                ("Protein", metrics.get("protein"), lambda v: f"{pct(v):6.2f} %"),
                ("Lactose", metrics.get("lactose"), lambda v: f"{pct(v):6.2f} %"),
                ("Solids", metrics.get("solids"), lambda v: f"{pct(v):6.2f} %"),
                ("Sweetness (sucrose eq)", metrics.get("sweetness_eq"), lambda v: f"{pct(v):6.2f} %"),
                ("Freezing point", metrics.get("freezing_point"), lambda v: f"{v:6.2f} °C"),
                ("Ice fraction @ serve", metrics.get("ice_fraction_at_serve"), lambda v: f"{v*100:6.2f} %"),
                ("Viscosity @ serve", metrics.get("viscosity_at_serve"), lambda v: f"{v:6.4f} Pa·s"),
                ("Overrun ceiling", metrics.get("overrun_estimate"), lambda v: f"{v*100:6.2f} %"),
                ("Hardness index", metrics.get("hardness_index"), lambda v: f"{v:6.2f}"),
                ("Meltdown index", metrics.get("meltdown_index"), lambda v: f"{v:6.2f}"),
                ("Lactose supersat.", metrics.get("lactose_supersaturation"), lambda v: f"{v:6.2f}"),
                ("Polymer solids", metrics.get("polymer_solids_pct"), lambda v: f"{v*100:6.3f} %"),
                ("Freezer load", metrics.get("freezer_load_kj"), lambda v: f"{v:7.1f} kJ"),
                ("Cost / kg", metrics.get("cost_per_kg"), lambda v: f"${v:6.3f}"),
            ]
            for label, value, formatter in prop_specs:
                if value is None:
                    continue
                lines.append(f"    {label:20s} {formatter(value)}")
        if self.notes:
            lines.extend(self.notes)
        return "\n".join(lines)

    def __str__(self) -> str:  # pragma: no cover - convenience for print()
        return self.__print__()

    def _named_weights(self) -> tuple[list[str], list[float], Dict[str, Ingredient]]:
        totals: OrderedDict[str, float] = OrderedDict()
        table: Dict[str, Ingredient] = {}
        for ing, weight in self.components:
            if weight <= 0:
                continue
            table[ing.name] = ing
            totals[ing.name] = totals.get(ing.name, 0.0) + weight
        keys = list(totals.keys())
        weights = [totals[name] for name in keys]
        return keys, weights, table

    def component_totals(self) -> Dict[str, float]:
        keys, weights, table = self._named_weights()
        return component_sums(keys, weights, table, symbolic=False)

    def formulation(self) -> Formulation:
        totals = self.component_totals()
        batch = totals["total"]
        if batch <= 0:
            raise ValueError("Recipe has zero total mass.")
        fat = totals["fat"] / batch
        protein = totals["protein"] / batch
        water = totals["water"] / batch
        snf = (totals["protein"] + totals["lactose"] + totals["ash"]) / batch
        sugars = {
            "sucrose": totals["sucrose"] / batch,
            "glucose": totals["glucose"] / batch,
            "fructose": totals["fructose"] / batch,
            "lactose": totals["lactose"] / batch,
            "polyols": totals["polyols"] / batch,
            "maltodextrin": totals["maltodextrin"] / batch,
        }
        stabilizer = (
            sum(
                weight * (ing.other_solids + ing.maltodextrin + ing.polyols)
                for ing, weight in self.components
                if ing.hydrocolloid
            )
            / batch
        )
        emulsifier = sum(weight for ing, weight in self.components if ing.emulsifier_power > 0) / batch
        return Formulation(
            milkfat_pct=fat,
            snf_pct=snf,
            water_pct=water,
            sugars_pct=sugars,
            stabilizer_pct=stabilizer,
            emulsifier_pct=emulsifier,
            protein_pct=protein,
        )

    def sweetness_pct(self) -> float:
        totals = self.component_totals()
        return sweetness_eq(totals) / totals["total"] if totals["total"] > 0 else 0.0

    def cost_per_kg(self) -> float:
        total = self.batch_mass_kg
        if total <= 0:
            return 0.0
        cost_total = sum(weight * ing.cost for ing, weight in self.components)
        return cost_total / total

    def _mix_metrics(
        self,
        *,
        serve_temp_C: float,
        draw_temp_C: float,
        shear_rate_s: float,
        overrun_cap: float | None = None,
    ) -> Dict[str, float]:
        keys, weights, table = self._named_weights()
        return build_properties(
            keys,
            weights,
            table,
            temp_C=serve_temp_C,
            draw_temp_C=draw_temp_C,
            shear_rate=shear_rate_s,
            symbolic=False,
            overrun_cap=overrun_cap,
        )

    def freezing_point(
        self,
        *,
        serve_temp_C: float,
        draw_temp_C: float,
        shear_rate_s: float,
        overrun_cap: float | None = None,
    ) -> float:
        metrics = self._mix_metrics(
            serve_temp_C=serve_temp_C,
            draw_temp_C=draw_temp_C,
            shear_rate_s=shear_rate_s,
            overrun_cap=overrun_cap,
        )
        return metrics["freezing_point"]

    def overrun_ceiling(
        self,
        *,
        serve_temp_C: float,
        draw_temp_C: float,
        shear_rate_s: float,
        overrun_cap: float | None = None,
    ) -> float:
        metrics = self._mix_metrics(
            serve_temp_C=serve_temp_C,
            draw_temp_C=draw_temp_C,
            shear_rate_s=shear_rate_s,
            overrun_cap=overrun_cap,
        )
        return metrics["overrun_estimate"]

    def mix_volume_L(
        self,
        *,
        serve_temp_C: float,
        draw_temp_C: float,
        shear_rate_s: float,
        overrun_cap: float | None = None,
    ) -> float:
        metrics = self._mix_metrics(
            serve_temp_C=serve_temp_C,
            draw_temp_C=draw_temp_C,
            shear_rate_s=shear_rate_s,
            overrun_cap=overrun_cap,
        )
        return metrics["volume_L"]

    def serving_size_for_volume(
        self,
        portion_L: float,
        *,
        serve_temp_C: float,
        draw_temp_C: float,
        shear_rate_s: float,
        overrun_cap: float | None = None,
    ) -> float:
        totals = self.component_totals()
        mix_volume = self.mix_volume_L(
            serve_temp_C=serve_temp_C,
            draw_temp_C=draw_temp_C,
            shear_rate_s=shear_rate_s,
            overrun_cap=overrun_cap,
        )
        if mix_volume <= 0:
            return 0.0
        density = totals["total"] / (mix_volume * (1.0 + self.overrun))
        return density * portion_L * 1000.0

    def nutrition_facts(
        self,
        serving_size_g: float,
        *,
        sodium_mg: float = 0.0,
    ) -> NutritionFacts:
        totals = self.component_totals()
        batch = totals["total"]
        if batch <= 0:
            raise ValueError("Recipe has zero total mass.")
        fat_pct = totals["fat"] / batch
        sugars_pct = (
            totals["sucrose"]
            + totals["glucose"]
            + totals["fructose"]
            + totals["lactose"]
        ) / batch
        protein_pct = totals["protein"] / batch
        snf_pct = (totals["protein"] + totals["lactose"] + totals["ash"]) / batch
        carbs_pct = sugars_pct + snf_pct - protein_pct
        trans_fat_pct = totals["trans_fat"] / batch if batch > 0 else 0.0
        cholesterol_mg_per_kg = totals["cholesterol_mg"] / batch if batch > 0 else 0.0
        saturated_fat_pct = totals.get("saturated_fat", 0.0) / batch
        added_sugars_pct = totals.get("added_sugars", 0.0) / batch

        fat_g = fat_pct * serving_size_g
        carbs_g = carbs_pct * serving_size_g
        protein_g = protein_pct * serving_size_g
        sugars_g = sugars_pct * serving_size_g
        trans_fat_g = trans_fat_pct * serving_size_g
        saturated_fat_g = saturated_fat_pct * serving_size_g
        added_sugars_g = added_sugars_pct * serving_size_g
        cholesterol_mg = cholesterol_mg_per_kg * (serving_size_g / 1000.0)
        calories = 9 * fat_g + 4 * carbs_g + 4 * protein_g
        return NutritionFacts(
            serving_size_g=serving_size_g,
            calories=calories,
            total_fat_g=fat_g,
            total_carbs_g=carbs_g,
            total_sugars_g=sugars_g,
            protein_g=protein_g,
            sodium_mg=sodium_mg,
            fat_pct=fat_pct,
            carbs_pct=carbs_pct,
            sugars_pct=sugars_pct,
            protein_pct=protein_pct,
            saturated_fat_g=saturated_fat_g,
            saturated_fat_pct=saturated_fat_pct,
            added_sugars_g=added_sugars_g,
            added_sugars_pct=added_sugars_pct,
            trans_fat_g=trans_fat_g,
            trans_fat_pct=trans_fat_pct,
            cholesterol_mg=cholesterol_mg,
            cholesterol_mg_per_kg=cholesterol_mg_per_kg,
        )


# --------------------------- Conversion helpers --------------------------- #


def recipe_to_formulation(recipe: Recipe) -> Formulation:
    """Helper that simply calls Recipe.formulation() for convenience."""

    return recipe.formulation()


def formulation_to_nutrition(
    formulation: Formulation,
    serving_size_g: float,
) -> NutritionFacts:
    """Compute a coarse nutrition panel from a formulation."""

    fat_pct = formulation.milkfat_pct
    carbs_pct = (
        sum(formulation.sugars_pct.values())
        + formulation.snf_pct
        - formulation.protein_pct
    )
    sugars_pct = (
        formulation.sugars_pct["sucrose"]
        + formulation.sugars_pct["glucose"]
        + formulation.sugars_pct["fructose"]
        + formulation.sugars_pct["lactose"]
    )
    protein_pct = formulation.protein_pct
    fat_g = fat_pct * serving_size_g
    carbs_g = carbs_pct * serving_size_g
    protein_g = protein_pct * serving_size_g
    sugars_g = sugars_pct * serving_size_g
    calories = 9 * fat_g + 4 * carbs_g + 4 * protein_g
    return NutritionFacts(
        serving_size_g=serving_size_g,
        calories=calories,
        total_fat_g=fat_g,
        total_carbs_g=carbs_g,
        total_sugars_g=sugars_g,
        protein_g=protein_g,
        fat_pct=fat_pct,
        carbs_pct=carbs_pct,
        sugars_pct=sugars_pct,
        protein_pct=protein_pct,
    )


__all__ = [
    "NutritionFacts",
    "Formulation",
    "ProductionSettings",
    "Recipe",
    "recipe_to_formulation",
    "formulation_to_nutrition",
]
