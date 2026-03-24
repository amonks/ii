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
- `blurb`: free text, may contain `#tags`
- `node_id`: which node last wrote this ping
- `updated_at`: LWW clock (unix nanos) — higher wins on merge
- `synced_at`: last sync timestamp

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
- **Push**: client sends unsynced pings to server via `POST /sync/push`
- **Pull**: client fetches changed pings via `GET /sync/pull?since=WATERMARK`
- **Period changes**: synced via `GET /sync/period-changes` (always sends all)
- LWW merge on receive: only apply if `incoming.updated_at > existing.updated_at`
- Periodic background sync (5 min) when upstream configured

## HTTP Routes

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/` | Dashboard: pending pings + recent history |
| GET | `/pings` | JSON: pending + recent pings (used by iOS) |
| POST | `/answer` | Set blurb for one ping |
| POST | `/batch-answer` | Batch-set blurb for multiple pings |
| GET | `/search?q=` | Full-text search |
| GET | `/graphs` | Time-by-tag charts |
| GET | `/graphs/data` | JSON histogram data |
| GET | `/settings` | Period/seed settings |
| POST | `/settings/period` | Add period change |
| POST | `/sync/push` | Receive pings from downstream |
| GET | `/sync/pull?since=` | Return changed pings |
| GET | `/sync/period-changes` | Return period changes |

## Graphs

Each ping represents `period_secs` of time. Tags are extracted from blurbs. Time-by-tag histograms are bucketed by hour/day/week. Rendered client-side with Canvas JS.

## iOS App

- Starts the Go node on localhost via gomobile
- Four tabs: Pings, Search, Graphs, Settings
- Pings tab is native SwiftUI with batch-set support
- Other tabs use WKWebView pointing at localhost
- Schedules up to 64 local notifications from the deterministic schedule
- On notification tap, opens to ping answer screen

## Storage

SQLite with WAL mode, raw `database/sql` (not GORM). Migrations via `pkg/migrate`. Database path: `$MONKS_DATA/tagtime.db`. Replicated to vault via litestream.

## Dependencies

- `modernc.org/sqlite` — pure Go SQLite
- `monks.co/pkg/migrate` — SQL migration runner
- `monks.co/pkg/serve` — HTTP mux
- Standard app infrastructure: `pkg/reqlog`, `pkg/tailnet`, `pkg/sigctx`, `pkg/gzip`, `pkg/database`
