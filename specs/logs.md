# Logs

## Overview

Centralized observability dashboard for the monks.co system. Ingests
structured JSON log events from all apps and provides a web UI for querying,
charting, and tracing them.

Code: [apps/logs/](../apps/logs/), [pkg/logs/](../pkg/logs/)

See also: [observability](observability.md) for the full pipeline.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/ingest` | Batch ingest raw JSON log events (no reqlog middleware to prevent loops) |
| GET | `/` | Analytics dashboard with chart, log table, and top pages |
| GET | `/query` | JSON API: chart query with group-by and filters for a time range |
| GET | `/events` | JSON API: paginated, filtered events for the log table |
| GET | `/values` | JSON API: distinct values for a dimension (populates filter dropdowns) |
| GET | `/trace/{id}` | Trace view: all events sharing a request ID |
| GET | `/index.css` | Compiled Tailwind CSS (embedded) |

## Data Model

### events

Raw event store. Each row has `id`, `timestamp`, `data` (raw JSON string),
plus SQLite **generated stored columns** extracted via `json_extract`:

`app`, `level`, `msg`, `request_id`, `method`, `host`, `path`, `route`,
`status`, `duration_ms`, `remote_addr`, `proxy_upstream`, `duration_bucket`
(log-scale bucket).

Indexed on `timestamp`, `request_id`, `app+timestamp`, `msg`,
`msg+timestamp`.

### daily_stats

Pre-aggregated counts by `(day, app, host, method, status,
duration_bucket)`. Upserted on every ingested `msg = "request"` event.

### page_daily

Pre-aggregated counts by `(day, host, path)`. Also upserted on ingest.

## Query Engine

`ParseQuery` parses a wire format like `group:host,app:homepage,!status:404`
into a `Query{GroupBy, Filters}` struct. Supported filter operators: `=`,
`!=`, `IN`, `NOT IN`.

`QueryChartData` adaptively selects the data source:
- < 7 days: hourly buckets from `events` table
- 7-179 days: daily buckets from `daily_stats`
- >= 180 days: weekly buckets

Dimensions not present in `daily_stats` (`proxy_upstream`, `level`,
`route`) always fall back to the `events` table.

## Frontend

Single-page dashboard using D3.js stacked bar charts. The query builder is
fully client-side; changing group-by or filters triggers `fetch()` calls to
`/query` and `/events`. Legend items are clickable to add/remove filters.

Color-codes remote addresses using `pkg/color.Hash` for deterministic
assignment.

## Deployment

Runs on **fly.io** (Chicago ORD). `shared-cpu-4x`, 2GB (larger than most
apps for SQLite query workload). Database at `/data/logs.db`. Access tier:
`tag:service` and `ajm@passkey`.
