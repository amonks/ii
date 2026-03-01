# Dogs

## Overview

Web gallery for a hot dog eating contest. Imports data from a shared Google
Spreadsheet, stores entries in SQLite, and presents a filterable/sortable
photo grid. Contestants: Monks, Ben, Chris, Fenn, Savely, ellie.

Code: [apps/dogs/](../apps/dogs/), [pkg/dogs/](../pkg/dogs/)

## Import Pipeline

The `Importer` runs a background loop every hour:

1. Checks a marker file (`<archiveDir>/last_import`) to skip redundant
   re-imports across restarts.
2. Downloads the Google Spreadsheet as a ZIP from a hardcoded URL.
3. Extracts the `🗄️ (DO NOT EDIT) Archive.html` sheet.
4. Parses with `goquery`: date, count, eater, photo, notes.
5. Computes `wordcount` from notes text.
6. Upserts all entries into SQLite via `ON CONFLICT UPDATE ALL`.
7. Downloads photos (from URL or extracted from ZIP) to
   `<archiveDir>/images/`.
8. Deletes stale images no longer referenced.

## Data Model

### entries

| Column | Type | Notes |
|--------|------|-------|
| `number` | integer PK | Entry number |
| `date` | text | |
| `count` | real | Hot dogs eaten |
| `eater` | text | Contestant name |
| `photo_url` | text | Original photo URL |
| `photo_filename` | text | Local filename |
| `notes` | text | Entry notes |
| `wordcount` | integer | Notes word count |

Indexes on `eater_date`, `date`, `wordcount`.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/images/` | Static file server for cached photos |
| GET | `/` | Gallery page with filter/sort support |

Query params: `combatants[]` (multi-select filter), `sortBy`
(number/count/wordcount), `sortOrder` (ascending/descending).

## Deployment

Runs on **fly.io** (Chicago ORD). `shared-cpu-2x`, 1GB. Database at
`/data/dogs.db`, images at `/data/images/`. Public access tier.
