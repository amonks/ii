# Logs Package

## Overview

SQLite-backed log storage and query engine for HTTP request events. Stores
raw JSON log lines and maintains pre-aggregated daily stats tables for
efficient charting. This is the data layer behind the [logs](logs.md) app.

Code: [pkg/logs/](../pkg/logs/)

## API

### Ingestion

- `Ingest(events []json.RawMessage) error` — bulk inserts raw JSON events.
  For each event with `msg = "request"`, upserts into both `daily_stats`
  and `page_daily` aggregate tables.

### Querying

- `QueryChartData(tr TimeRange, q Query) ([]ChartPoint, int64, error)` —
  returns time-series data. Adaptively selects: hourly from `events` (<7d),
  daily from `daily_stats` (7-179d), weekly (>=180d).
- `GetRecentEvents(tr TimeRange, limit int) ([]Event, error)`
- `GetFilteredEvents(tr TimeRange, q Query, limit, offset int) ([]Event, error)`
- `GetDimensionValues(tr TimeRange, dim string) ([]string, error)`
- `GetTrace(requestID string) ([]Event, error)`

### Query Parsing

- `ParseQuery(s string) Query` — parses wire format:
  `group:host,app:homepage,!status:404`
- `Query.FormatQuery() string` — serializes back to wire format.
- `ParseTimeRange(req *http.Request) TimeRange` — parses `?range=7d` or
  `?start=&end=`.

## Schema

See [logs app spec](logs.md) for the full table definitions.

## Dependencies

- `pkg/database` — GORM/SQLite wrapper
