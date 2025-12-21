# Plan

Refactor the creamery domain model to distinguish canonical specs, measured lots, and solver/recipe usage while centralizing composition math and richer targets.

## Requirements
- Introduce clear IngredientSpec vs IngredientLot ownership and eliminate duplicated constituent data.
- Ensure the ingredient catalog, solver, recipe builder, and analysis layers all consume the same canonical types.
- Normalize composition/snapshot/label math around ConstituentProfile so every conversion flows through one path.
- Expand FormulationTarget to profile-shaped intervals with upfront validation and keep all callers compiling.
- Preserve current behaviors (tests/examples) or update expectations where semantics change.

## Scope
- In: domain structs, catalog construction, solver/problem wiring, recipe aggregation, label/fda conversions, relevant tests/CLI utilities.
- Out: external APIs/CLI flags beyond what breaks from refactor, new ingredient data beyond reshaping existing table, perf optimization.

## Files and entry points
- `domain.go`, `ingredient.go`, `analysis_ingredients.go` for core types/catalog data.
- `problem.go`, `solver.go`, `sample.go` for optimization wiring.
- `analysis_recipe.go`, `analysis_chemistry.go`, `label*.go` for recipe/snapshot/label conversions.
- `target.go`, `fda.go`, `label_scenarios.go` for FormulationTarget usage.
- Tests under `*_test.go` and CLI commands in `cmd/`.

## Data model / API changes
- Define `IngredientSpec` (identity + canonical profile) and `IngredientLot` (spec ref + lot metadata) to replace overloaded structs.
- Update catalog to own specs and their default lot(s), returning cohesive `SpecLot` handles.
- Provide helper views on `ConstituentProfile`/`BatchSnapshot` for legacy `Composition`, `NutritionFacts`, etc.
- Represent formulation targets as profile-shaped intervals (fat, protein, lactose, sugars, water, POD/PAC) with validation logic.

## Action items
[x] 1. Introduce new IngredientSpec/IngredientLot structs, migrate constructors/catalog loading, and adapt constants/importers.
[x] 2. Rebuild IngredientCatalog with explicit spec/lot registries and update consumers (StandardSpecs, SpecFromComposition, etc.).
[x] 3. Update Problem/Solver/Solution to operate on the new spec references and carry lot metadata through to downstream builders.
[x] 4. Refactor RecipeComponent, BatchSnapshot aggregation, and analysis helpers to consume IngredientLot data directly.
[x] 5. Add canonical conversion helpers on ConstituentProfile/BatchSnapshot (to Composition, NutritionFacts, etc.) and replace ad-hoc math.
[ ] 6. Expand FormulationTarget into profile-interval form with validation; update FDA/label ingestion and solver constraint wiring.
[ ] 7. Refresh CLI tools/tests/docs to compile with the new model and document the updated domain types.

## Testing and validation
- Re-run existing Go tests (`go test ./...`).
- Exercise representative CLI commands (e.g., `cmd/label-scenarios`) if affected.
- Spot-check example workflows (`example_test.go`, `workflow_test.go`) for behavioral regressions.

## Risks and edge cases
- Large refactor may introduce subtle mismatches between spec and lot data; rely on compiler errors and targeted tests.
- Solver constraints may double-count or miss components if interval wiring is off; ensure validation covers impossible targets early.
- Label/Nutrition calculations must remain accurate; cross-check against prior snapshots for baseline recipes.

## Open questions
- None; assumptions documented in plan and can be revisited during implementation.
