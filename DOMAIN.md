# Creamery Domain Model Redesign

## Purpose
Unify all source-of-truth ingredient data, lots, and blends around a small set of immutable types so every subsystem (catalog, solver, recipe, labeling) reads the same normalized structures. The goals are:
- Centralize ingredient definitions and eliminate ad-hoc copies spread across specs, lots, and solver entries.
- Separate immutable definitions from batch-specific data (lots, blends, snapshots) so caching and overrides are predictable.
- Provide explicit types for intermediate artifacts (Blend, BatchReport) so conversion logic is shared instead of reimplemented.

## Core Entities

### IngredientDefinition
```go
type IngredientDefinition struct {
    ID      IngredientID      // normalized, unique, immutable
    Key     IngredientKey     // optional canonical catalog key
    Name    string            // preferred human name
    Profile ConstituentProfile// fully normalized; no zero IDs or names
}
```
**Invariants**
- `ID` is non-empty and globally unique; derived via `NewIngredientID` if not provided.
- `Profile` belongs to the same `ID`/`Name`; `Profile.Components.Validate()` always succeeds.
- Instances are immutable after creation; reuse pointers to share across the catalog, problems, and recipes.

### LotDescriptor
```go
type LotDescriptor struct {
    Definition *IngredientDefinition
    DisplayName string
    LotCode     string
    ProfileOverride *ConstituentProfile // optional, normalized when set
}
```
**Invariants**
- `Definition` must be non-nil; it is never cloned.
- `DisplayName` defaults to `Definition.Name`; when explicitly set it does **not** change the definition.
- `ProfileOverride`, when present, matches the definition ID/name. `EffectiveProfile()` returns `ProfileOverride` if set, otherwise the definition profile.

### IngredientCatalog
```go
type IngredientCatalog struct {
    defsByID map[IngredientID]*IngredientDefinition
    lotsByID map[IngredientID]LotDescriptor // default lots (share Definition pointers)
    keyIndex map[IngredientKey]IngredientID
}
```
**Invariants**
- Catalog owns all `IngredientDefinition` instances; lots and later structures reference by pointer.
- `defsByID` and `lotsByID` contain the same keys; missing entries indicate an unknown ingredient.
- `keyIndex` only stores canonical keys (snake_case) and maps to IDs present in `defsByID`.

### Blend & BatchReport
```go
type Blend struct {
    Components []BlendComponent // ordered, sum of weights == 1 for fractions
}

type BlendComponent struct {
    Definition *IngredientDefinition
    Lot        LotDescriptor // optional lot metadata (may be zero)
    Weight     float64       // kg or fraction, depending on context
}

func (b Blend) TotalWeight() float64
func (b Blend) AsFractions() Blend
```
`Blend` is the canonical bridge between solver output and recipe/build analyses.

```go
type BatchReport struct {
    Blend        Blend
    Constituents ConstituentComponents // aggregated via weights
    Sweeteners   SweetenerAnalysis
    Economics    BatchEconomics
    Physics      ProcessMetrics
}
```
**Invariants**
- `Components` always equals the weighted sum of each component profile in the blend.
- `Sweeteners` derives solely from the aggregated blend; no downstream recalculation.
- Report creation is pure: given the same blend and options, results are deterministic.

### Targets & Constraints
```go
type FormulationTarget struct {
    Components ConstituentComponents // canonical view
    Constraints TargetConstraints
}

type TargetConstraints struct {
    Water Interval
    POD   Interval
    PAC   Interval
    CompositionLegacy *Composition // optional, derived from Components when nil
}
```
**Invariants**
- Only one canonical components struct is stored; legacy `Composition` is derived on demand.
- Validation lives on `ConstituentComponents` plus targeted extras (water, POD, PAC).

## Relationships
1. **Catalog** owns immutable `IngredientDefinition` instances and publishes zero or more `LotDescriptor` defaults.
2. **Problem** stores `[]*IngredientDefinition` plus optional lot overrides but never copies profiles. It references `FormulationTarget` for goals.
3. **Solver** returns a `Blend` (weights + definition pointers) instead of loose maps; `Solution` becomes a thin wrapper around `Blend` plus metadata.
4. **Recipe/Labeling** consume the same `Blend`, build a `BatchReport`, and annotate user-visible artifacts (recipes, labels, cost sheets) without recomputing compositions from scratch.

## Normalization Rules
- All ingestion paths (`SpecFromProfile`, catalog builders, ad-hoc compositions) funnel through `NewIngredientDefinition(profile ConstituentProfile, key IngredientKey)` which guarantees normalized IDs, names, and keys.
- `LotDescriptor` construction requires a `*IngredientDefinition`; helper `NewLot(def *IngredientDefinition)` copies pointers but never structs.
- `Blend` APIs enforce: `Weight >= 0`, total weight > 0, and provide helpers to convert between kg and fractions.

## Migration Notes
- Replace `IngredientSpec` with `IngredientDefinition` everywhere; `IngredientLot` becomes `LotDescriptor`.
- Update caches (`Problem.entries`, `IngredientCatalog.lots`, solver coefficient builders) to keep pointers instead of value copies.
- Introduce `Blend` and `BatchReport` iteratively: first use it inside solver output, then propagate to recipes and batch analysis to remove redundant aggregation logic.
