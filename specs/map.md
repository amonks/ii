# Map

## Overview

Interactive Google Map of places the owner has visited, seeded from a Google
Takeout export. Users can filter by venue type and click markers for details.

Code: [apps/map/](../apps/map/)

## Data Import (Offline)

Data is imported via CLI commands in `cmd/mapctl`, not through the running
server:

1. Download Google Takeout export ("Maps - Your Places - Saved").
2. `go run ./cmd/mapctl -operation=import-saved-places`: calls Google Places
   API per place to enrich with business details. Skips places that already
   have both a `GoogleMapsPlaceID` and `EditorialSummary`.
3. `go run ./cmd/mapctl -operation=annotate-peoples-places`: reads
   `people.csv` to annotate places with "X was here" notes.

## Data Model

### places

| Column | Type | Notes |
|--------|------|-------|
| `google_maps_url` | text PK | |
| `google_maps_place_id` | text | |
| `google_maps_business_status` | text | OPERATIONAL/CLOSED_TEMPORARILY/CLOSED_PERMANENTLY |
| `lat`, `lng` | real | Coordinates |
| `business_name`, `address`, `title` | text | |
| `country_code` | text | |
| `notes`, `rating` | text | |
| `is_public` | bool | |
| `editorial_summary` | text | From Google Places API |
| `opening_hours` | text | JSON array |
| `types` | text | JSON array |
| `price_level` | integer | |
| `details_json` | text | Full raw API response |

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Map with all markers and filtering, or detail panel if `?url=` is set |
| GET | `/index.js` | Embedded compiled TypeScript |
| GET | `/index.css` | Embedded compiled Tailwind CSS |
| GET | `/dot.png` | Map marker image |

The detail panel is loaded via HTMX on marker click. Venue types: bars,
cafes, restaurants, stores, other. Closed places show a red badge.

## Frontend

Google Maps JS API renders the interactive map (requires
`GOOGLE_PLACES_BROWSER_API_KEY` env var). The map is centered on H Mart
Chicago. Uses htmx for marker detail loading and hyperscript for filter
toggles.

## Deployment

Runs on **fly.io** (Chicago ORD). Database at `/data/map.db`. Public access
tier.
