# TagTime

Stochastic time tracking via Poisson process sampling.

## Overview

TagTime prompts the user at random intervals to record what they're doing. The intervals come from a deterministic seeded Poisson process, so every client converges to the same schedule. Users respond with short blurbs containing #tags, then view how their time is distributed across tags over configurable windows.

## Architecture

Same pattern as breadcrumbs: a Go core in `node/` shared between:
- A **server** (`main.go`) running on the tailnet
- An **iOS app** (`ios/`) via gomobile (`mobile/mobile.go`)

All interaction with the node is via HTTP, even on the phone (localhost).

## Data Model

### Pings

Keyed by unix-second timestamp (deterministic from schedule). Each ping has:
- `blurb`: free text, may contain `#tags` — **immutable once written** (never rewritten by renames)
- `node_id`: which node last wrote this ping
- `updated_at`: LWW clock (unix nanos) — higher wins on merge
- `synced_at`: last sync timestamp
- `received_at`: server-assigned timestamp (unix nanos) set when ping arrives via push — used for watermark-based pull filtering

### Tags

Tag names may contain word characters, `/`, and `.` to form hierarchies (e.g. `#coding/monks.co/tagtime`). The tag regex is `#(\w(?:[\w./]*\w)?)` — tags must start and end with a word character, with `/` and `.` allowed in the middle.

Structured tag data lives alongside blurbs:

- **`tags` table**: all known tag names (PK: `name`). Populated automatically when blurbs are written. Used for autocomplete.
- **`ping_tags` table**: association table mapping `(ping_timestamp, tag_name)`. Derived from blurbs on write. Updated by renames. Reconciled on every write: stale entries not matching the current blurb are deleted.
- **`tag_renames` table**: rename event log — `(old_name, renamed_at)` PK, plus `new_name` and `node_id`.

Tags in `ping_tags` reflect the canonical post-rename name. The original blurb text is never modified.

### Tag Renames (Time-Scoped)

Renames are scoped by time: a rename at time T only affects `ping_tags` rows where `ping_timestamp <= T`. This allows the same tag name to be reused after a rename.

Example:
- T1: user adds `#sleep` → ping_tags(T1, "sleep")
- T3: user renames `sleep → sleeping` → ping_tags(T1, "sleeping")
- T4: user adds `#sleep` again → ping_tags(T4, "sleep") — NOT renamed

Both `sleep` and `sleeping` coexist as valid tags.

On sync, renames are applied idempotently: `UPDATE ping_tags SET tag_name = new WHERE tag_name = old AND ping_timestamp <= renamed_at`.

### Period Changes

Event-sourced log of schedule parameter changes:
- `timestamp`: when this change takes effect (unix seconds)
- `seed`: PRNG seed
- `period_secs`: average gap between pings

The schedule is regenerated from the full event log. All nodes converge.

### Full-Text Search

SQLite FTS5 virtual table on ping blurbs, kept in sync via triggers.

## Schedule Algorithm

Seeded PCG PRNG. Each inter-ping gap drawn from exponential distribution: `gap = -period * ln(1 - rand.Float64())`. Average gap defaults to 45 minutes (~32 pings/day).

Period changes are event-sourced: to generate pings across a time range, walk the period change log and generate pings for each active segment.

## Sync

Star topology, watermark-based:
- **Push**: client sends unsynced pings, all period changes, and all tag renames to server via `POST /sync/push`. Server stamps `received_at` (unix nanos) on each ping at receipt time, derives `ping_tags` from blurbs, and applies received tag renames.
- **Pull**: client fetches changed pings, period changes, and tag renames via `GET /sync/pull?since=WATERMARK`. The watermark tracks server-assigned `received_at` values (not `updated_at`), ensuring late-arriving pings from offline clients are visible to all peers. On pull, the client derives `ping_tags` from received blurbs and applies received tag renames.
- **Sync Now**: `POST /sync/now` triggers an immediate push+pull cycle (used by the iOS sync button).
- LWW merge on receive: only apply if `incoming.updated_at > existing.updated_at`
- Period changes and tag renames are idempotent (keyed by primary key), so sending all on every push/pull is safe
- Periodic background sync (5 min) when upstream configured
- In-memory period change cache refreshes immediately on settings change or sync push
- Sync status tracks `last_push_at` and `last_pull_at` timestamps for UI display

## HTTP Routes

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/` | Dashboard: next ping countdown, pending pings, recent history (editable) |
| GET | `/pings` | JSON: pending + recent pings (used by iOS) |
| POST | `/answer` | Set blurb for one ping |
| POST | `/batch-answer` | Batch-set blurb for multiple pings |
| GET | `/tags` | JSON: all known tag names, sorted |
| GET | `/tags/summary?range=` | JSON: tags ranked by time spent with sparklines and hierarchical tree (range: `24h`, `7d`, `30d`, `all`) |
| GET | `/tags/{name...}` | JSON: tag detail — rename history and pings containing this tag (wildcard path for hierarchical names) |
| POST | `/tags/rename` | Rename a tag (time-scoped): `old_name`, `new_name` |
| GET | `/search?q=` | Full-text search (HTML) |
| GET | `/search/data?q=` | Full-text search (JSON) |
| GET | `/graphs` | Time-by-tag charts |
| GET | `/graphs/data` | JSON histogram data (includes `tag_colors`) |
| GET | `/settings` | Period/seed settings |
| POST | `/settings/period` | Add period change |
| POST | `/sync/push` | Receive pings from downstream |
| GET | `/sync/pull?since=` | Return changed pings |
| POST | `/sync/now` | Trigger immediate push+pull |
| GET | `/sync/status` | Sync status (upstream, unsynced count, timestamps) |
| GET | `/sync/period-changes` | Return period changes |

## Tag Autocomplete

Both iOS and web UIs provide tag autocomplete:
- **iOS**: `TagTextField` component fetches `/tags` on load, shows filtered suggestion chips in a horizontal scroll view as the user types after `#`. Tapping a chip inserts the completed tag.
- **Web**: `<datalist>` element populated from `/tags` on page load, attached to all blurb input fields.

## Tags Tab

The Tags tab (iOS) shows a hierarchical tree view of tags for a selectable time range. Tags containing `/` (e.g. `coding/monks.co/tagtime`) are displayed as a collapsible tree with `DisclosureGroup`. Parent nodes aggregate child time and sparklines. Each row displays the segment name, total time (own + descendants), and an inline sparkline histogram. Tapping a tag opens a detail page with rename support, rename history, and the list of pings containing that tag.

The `/tags/summary` endpoint computes per-tag time totals and sparkline data, plus a `tree` field containing the hierarchical `TagTreeNode` structure. Each ping's contribution is weighted by the effective `period_secs` at that ping's timestamp (from the period change log), so time accounting remains accurate across period changes. Sparklines use 20 fixed-width sub-buckets of absolute seconds. The flat `tags` array is sorted by total time descending; the `tree` is sorted by total time at each level.

Tree nodes have `own_secs` (time from direct pings to that exact tag) and `total_secs` (own + all descendants). Pure aggregator nodes (parents with no direct pings) have `own_secs = 0`. The tree is built by `BuildTagTree()` in `tag_tree.go`.

The `/tags/{name...}` detail endpoint returns all renames involving the tag (as old_name or new_name) and all pings containing the tag. Uses a wildcard path to support hierarchical tag names with `/`.

## Graphs (Web)

Tags are resolved from `ping_tags` (the structured, post-rename association table). Time-by-tag histograms are bucketed by hour/day/week. Each bucket's tag values are **percentages (0-100)** of time within that bucket, weighted by the effective `period_secs` at each ping. Using percentages rather than absolute time ensures charts remain comparable across period changes. The graph data endpoint includes a `tag_colors` map with canonical hex colors from `pkg/color`, so all clients render consistent tag colors. Rendered client-side with Canvas JS (web).

## iOS App

- Starts the Go node on localhost via gomobile
- Four tabs: Pings, Search, Tags, Settings
- Pings tab is native SwiftUI with batch-set support, tap-to-edit on recent pings, and tag autocomplete
- Search tab uses server-side FTS via `/search/data` JSON endpoint
- Tags tab shows hierarchical tree view with collapsible nodes, sparklines, and aggregated time via `/tags/summary`, with drill-down to tag detail via `/tags/{name...}`
- Settings tab shows notification preferences (sound, time-sensitive delivery), next ping countdown, period display/change, sync controls
- Schedules up to 64 local notifications from the deterministic schedule; notification sound and interruption level are configurable via settings (stored in UserDefaults)
- On notification tap, opens to ping answer screen

## Storage

SQLite with WAL mode, raw `database/sql` (not GORM). Migrations via `pkg/migrate`. Database path: `$MONKS_DATA/tagtime.db`. Replicated to vault via litestream.

## Dependencies

- `modernc.org/sqlite` — pure Go SQLite
- `monks.co/pkg/migrate` — SQL migration runner
- `monks.co/pkg/serve` — HTTP mux
- `monks.co/pkg/color` — deterministic tag color hashing
- Standard app infrastructure: `pkg/reqlog`, `pkg/tailnet`, `pkg/sigctx`, `pkg/gzip`, `pkg/database`
