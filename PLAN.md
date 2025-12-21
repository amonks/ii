# Plan

Rebuild the domain model around immutable ingredient definitions, lightweight lots, and explicit blends so every subsystem (catalog, solver, recipe, labeling) shares the same normalized structures.

## Requirements
- Introduce `IngredientDefinition`/`LotDescriptor` types with documented invariants and normalization helpers.
- Migrate the catalog, problem setup, solver, and analysis layers to reference definitions by pointer instead of copying specs.
- Add a `Blend` aggregate that solver outputs and downstream consumers reuse for batch analytics.
- Remove redundant composition/profile copies and ensure tests reflect the new model.

## Scope
- In: domain types, catalog construction, solver/problem wiring, batch/recipe/label plumbing, associated tests and fixtures.
- Out: CLI UX, label scenario definitions, new physics models, external APIs (only internal domain modeling).

## Files and entry points
- `domain.go`, `ingredient.go`, new shared domain files for definitions/lots/blends.
- `problem.go`, `solver.go`, `batch_profile.go`, `analysis_recipe.go`, `analysis_chemistry.go`.
- Tests touching specs/lots (`*.test.go`).

## Data model / API changes
- `IngredientSpec` → `IngredientDefinition` (immutable pointer semantics).
- `IngredientLot` → `LotDescriptor` referencing a definition; overrides are explicit.
- Catalog exposes `Definitions()` and `DefaultLots()` views backed by shared pointers.
- `Problem` stores definitions plus optional lot overrides; `Solution` exposes a `Blend` instead of loosely coupled maps.
- `Blend` feeds a reworked `BatchReport` builder that replaces ad-hoc aggregation helpers.

## Action items
[ ] **Step 1 — Introduce core types:** add `IngredientDefinition`, `LotDescriptor`, normalization helpers, and constructor functions; keep legacy types temporarily aliased for compatibility.
[ ] **Step 2 — Rebuild catalog:** refactor catalog construction to emit definitions/lots using the new types, update `IngredientProfileTable` consumers, and adjust standard spec helpers.
[ ] **Step 3 — Update Problem ingestion:** store definition pointers + lot descriptors in `Problem`, expose helper accessors, and adapt tests/consumers (weight bounds, override logic).
[ ] **Step 4 — Solver integration:** change solver setup to read definitions directly, ensure coefficient extraction pulls from definition/lot pointers, and update `Solution` to wrap a `Blend`.
[ ] **Step 5 — Batch/recipe plumbing:** introduce the `Blend` struct plus aggregation helpers, update `BatchProfile`, `Recipe`, and label utilities to consume blends instead of rebuilding component maps.
[ ] **Step 6 — Clean up legacy structures:** remove old spec/lot structs, rename files, run targeted gofmt/go test, and adjust docs/examples to reference the new domain terminology.

## Testing and validation
- `go test ./...` after each major refactor (at least after steps 3, 5, and 6).
- Existing unit tests for catalog, solver, and recipes should pass with minimal changes; add new tests that assert pointer identity/invariants for definitions and lots.

## Risks and edge cases
- Pointer sharing bugs (mutating a definition after reuse) — enforce immutability via unexported setters and defensive copies when necessary.
- Widespread type renames could cascade; plan includes incremental aliasing to keep compiler churn manageable.
- Batch analytics rely on precise totals; ensure `Blend` aggregation keeps numerical parity with previous implementation.

## Open questions
- None; assumptions are documented in DOMAIN.md and can be revisited after implementation.
