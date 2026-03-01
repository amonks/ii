# Calendar

## Overview

Personal TV show tracker with a weekly calendar view. Tracks which TV
episodes have been watched each week via a simple client-side web app
backed by a flat JSON key-value store.

Code: [apps/calendar/](../apps/calendar/)

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Serves the embedded `index.html` single-page app |
| GET | `/storage` | Returns all stored data as JSON |
| POST | `/storage` | Merges a `map[string]string` JSON body into storage, persists to disk |

## Data Storage

A flat JSON file at `data/storage.json`, loaded into a
`sync.RWMutex`-protected `map[string]string`. Writes are atomic (write to
`.tmp`, then `os.Rename`).

The only key used is `tvShows`, storing a JSON array of show objects:
```json
[{ "name": "Show Name", "day": "Tuesday", "lastWatched": "2026-02-25" }]
```

## Frontend Logic

All application logic lives in a `<script type="module">` block in
`index.html`:

- Renders a 7-column table showing the last 7 days (today on the right).
- Shows are assigned to a day of the week. Each week a show appears in the
  column matching its air day.
- Watch state is tracked via `lastWatched`: a show is "watched" if
  `lastWatched >= cellDate`. Toggling either advances to the cell date or
  rolls back by 7 days.
- Add show modal records name and day-of-week, initializing `lastWatched`
  to the most recent past occurrence of that weekday.

## Deployment

Runs on **brigid** (local server). Private access tier.
