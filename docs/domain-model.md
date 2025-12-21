# Creamery Domain Model

## Ingredients
- **IngredientSpec** – canonical definition of an ingredient. It stores a stable `IngredientID`, display name, and a `ConstituentProfile` (fat/MSNF/sugar/other plus nutritional + functional metadata). Specs live in catalogs and are used when defining optimization problems.
- **IngredientLot** – a measured or vendor-specific lot tied to a spec. Lots keep the spec reference, may override the profile (e.g., lab analysis adjustments), and carry lot-specific metadata (display name, lot code, economics). Recipe components now always reference lots, and solver solutions preserve the lots they solved with.
- **IngredientCatalog** – registry that owns both specs and their default lots. `DefaultIngredientCatalog` is built from the canonical `IngredientProfileTable`, so callers can fetch either the canonical spec or a ready-to-use lot via `Instance`/`InstanceByKey` without going through the old `IngredientBatch` conversion path.

## Problems and Solutions
- `Problem` stores specs plus an ordered slice of lots; the solver coefficients use each lot's effective `ConstituentProfile`. Custom lots (from lab data or label reconstruction) can be injected with `OverrideLots`, which keeps the slice, map, and coefficient cache in sync.
- `Solution` exposes `Weights`, `Names`, and the `Lots` map, and now carries a `Components` field with aggregated `ConstituentComponents` plus the derived `Achieved` composition. Downstream code (labels, CLI tools) can rely on the richer constituent data instead of re-summing four-part compositions.

## Batch Snapshots
- `BatchSnapshot` aggregates per-component totals using the lot data. New helpers:
  - `FormulationBreakdown()` returns the lightweight `Formulation` summary (milkfat, SNF, sugars, stabilizer/emulsifier pct) along with the canonical `ConstituentComponents` snapshot used by reporting tools.
  - `NutritionFactsSummary()` generates per-serving `NutritionFacts`, mirroring `Recipe.NutritionFacts` but reusable by other tools/tests.

## Targets
- `FormulationTarget` mirrors a profile: it keeps the legacy `Composition` plus a `ConstituentComponents` interval set and explicit `Water`, `POD`, and `PAC` ranges. Helper accessors (`ProteinInterval`, `AddedSugarsInterval`, etc.) feed solver constraints, and `Validate()` ensures every interval is sane before solving.
- FDA/label ingestion (`NutritionLabel.ToTarget`) populates these intervals so the solver operates on consistent component-level data instead of ad-hoc fields.
