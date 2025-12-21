# Batch Log Format

Batch logs live in the `batchlog` file (feel free to split by location/project if needed). Each record captures ingredient weights plus free-form process/tasting notes. The syntax is inspired by YAML but is purpose built for fast editing: every field starts with `key:` on its own line, and any indented lines immediately following that key belong to the same value.

## Record structure

- Records are separated by a blank line or a line that starts with `%%`.
- Lines beginning with `#` are ignored.
- Top-level keys must start at column 0. Any line indented by spaces/tabs belongs to the most recent key and is treated as part of a multi-line value.
- `ingredients:` is a special block field that expects `- weight ingredient_key` entries.
- Block values keep their internal line breaks; the parser trims the common indentation so relative indent (e.g., sub-bullets) is preserved.

### Supported fields

| Key             | Description                                                                                              |
|-----------------|----------------------------------------------------------------------------------------------------------|
| `date`          | ISO-8601 date (`YYYY-MM-DD`). Required.                                                                   |
| `recipe`        | Free-form recipe or reference name. Optional.                                                             |
| `ingredients`   | Indented block of `- 320g sucrose` style lines. Each entry consists of a weight + catalog key.           |
| `process_notes` | Indented block representing one multi-line string. Blank lines inside the block become paragraph breaks. |
| `tasting_notes` | Same as `process_notes`, but for sensory feedback.                                                        |

Any other keys are preserved under `BatchLogEntry.Metadata` so future tools can consume them.

### Example entry

```
date: 2025-12-21
recipe: cream36_baseline
ingredients:
  - 0.432 kg cream36
  - 0.2669 kg whole_milk
  - 0.1115 kg skim_milk_powder
  - 0.1895 kg sucrose
  - 0.0002 kg avacream
process_notes:
  Added ~1/8 tsp Avacream (logged as 0.2 g estimate)
  circulator 69C 30m in zippy
    not great; impossible to get the air out, plus it insists on being not-flat; core temp almost certainly too low
  homogenize 45s in vitaprep
  fridge 24h
  prechill freezer 15m
  freeze, draw at sampled -4C after roughly 12m (too slow; should chill longer next time)
tasting_notes:
  tastes kind of cooked; should pasteurize lower temp or add solids after pasteurization
  chewy; solids seem very high
```

## CLI workflow

Use the `batch-log` command to parse and analyze the log:

```
go run ./cmd/batch-log --log ./path/to/batchlog
```

It prints aggregate stats to stdout and, when invoked with `--serve :8080`, starts a local HTTP server that renders the same analytics as an HTML dashboard. The CLI reuses the shared domain model, so ingredient keys must match the canonical catalog (`DefaultIngredientCatalog`).

## Tips

- Stick to canonical ingredient keys (`creamery.DefaultIngredientCatalog().InstanceByKey`) so computed chemistry matches production reality.
- Always include units on weights. Bare numbers are interpreted as kilograms.
- Prefer ISO dates so batches can be sorted lexicographically.
