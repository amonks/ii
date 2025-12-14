"""Modular ice cream formulation solver (CasADi/IPOPT)."""

from .chemistry import build_properties
from .ingredients import Ingredient, default_ingredients, costs_from
from .label import (
    LabelGroup,
    goals_from_label,
    apply_group_bounds,
    apply_label_order,
)
from .analysis import (
    NutritionFacts,
    Formulation,
    Recipe,
    recipe_to_formulation,
    formulation_to_nutrition,
)
from .model import (
    Constraint,
    FormulationGoals,
    FormulationProblem,
    FormulationSolution,
    GoalWeights,
    solve_formulation,
)

__all__ = [
    "Ingredient",
    "default_ingredients",
    "costs_from",
    "Constraint",
    "FormulationGoals",
    "FormulationProblem",
    "FormulationSolution",
    "GoalWeights",
    "solve_formulation",
    "LabelGroup",
    "goals_from_label",
    "apply_group_bounds",
    "apply_label_order",
    "build_properties",
    "NutritionFacts",
    "Formulation",
    "Recipe",
    "recipe_to_formulation",
    "formulation_to_nutrition",
]
