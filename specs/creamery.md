# Creamery

## Overview

Ice cream formulation app — models ingredients, solves for target
compositions, reconstructs commercial label recipes, and tracks a
batch log of production runs with composition analytics.

Code: [apps/creamery/](../apps/creamery/)

## CLI Commands

| Command | Description |
|---------|-------------|
| `serve` | Start the web console (tailnet HTTP server) |
| `labels` | Analyze all commercial label reconstructions |
| `recipes` | Analyze recipes from batch log and recipe files |
| `notebook` | Run the workflow/notebook sandbox |
| `substitute` | Substitute ingredients in a recipe using the solver |

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Overview dashboard with links to sections |
| GET | `/labels/` | Label reconstruction results with solver diagnostics |
| GET | `/recipes/` | Recipe catalog with composition and tasting notes |
| GET | `/batchlog/` | Batch log analytics: ingredient usage, timeline |

## Key Concepts

- **Ingredient catalog**: Built-in database of ingredient compositions
  (fat, protein, sugars, water, etc.) with cost data.
- **Label reconstruction**: Reverse-engineers commercial ice cream recipes
  from FDA nutrition facts and ingredient lists using constrained
  optimization (NLopt solver).
- **Batch log**: Directory of `.batch` files recording production runs
  with ingredients, process notes, and tasting notes.
- **Composition snapshot**: Aggregated nutritional profile computed from
  ingredient weights — fat%, MSNF%, sugars, freezing point, etc.

## Deployment

Runs on **brigid** (local server). Private access tier.
