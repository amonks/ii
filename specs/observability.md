# Observability

## Overview

monks.co uses a centralized logging pipeline where every app emits
structured JSON log events that are forwarded to the `logs` app over the
tailnet and stored in SQLite for querying. Uptime monitoring uses Dead Man's
Snitch heartbeat pings. Prometheus metrics are exported by the proxy.

## Logging Pipeline

### Request Logging (`pkg/reqlog`)

Every app calls `reqlog.SetupLogging()` at startup, which configures:

1. A `slog.JSONHandler` writing to stderr for local log output.
2. A `logsclient.Client` that buffers log lines and ships them to the logs
   service over the tailnet.

The `reqlog.Middleware()` HTTP middleware wraps each request in a "wide
event" pattern: attributes are accumulated throughout request handling via
`reqlog.Set(ctx, key, value)`, and a single structured log event is emitted
when the response completes. Fields include method, host, path, route,
status code, duration, remote address, and request ID.

### Log Shipping (`pkg/logsclient`)

`logsclient.Client` implements `io.Writer` and buffers JSON log lines. It
flushes to the logs service:
- Every 5 seconds, or
- When the buffer reaches 100 events

Logs are shipped via `POST http://monks-logs-fly-ord/ingest` using the
tailnet HTTP client. The client waits for the tailnet to be ready before
attempting to ship.

### Log Storage (`pkg/logs` + `apps/logs`)

The logs app ingests raw JSON events into a SQLite `events` table with
generated stored columns extracted via `json_extract` for indexed querying:
`app`, `level`, `msg`, `request_id`, `method`, `host`, `path`, `route`,
`status`, `duration_ms`, `remote_addr`, `proxy_upstream`, and
`duration_bucket`.

Two pre-aggregated tables are maintained for fast charting:
- `daily_stats`: counts by `(day, app, host, method, status, duration_bucket)`
- `page_daily`: counts by `(day, host, path)`

Aggregates are updated on every ingest via upsert. The query engine
adaptively selects hourly buckets from raw events for ranges < 7 days,
daily buckets from `daily_stats` for 7-179 days, and weekly buckets for
longer ranges.

### Self-Referential Protection

The logs app's `/ingest` route is mounted **without** the `reqlog`
middleware to prevent a log-shipping loop.

### Event Types

The logging system uses four `msg` values as a taxonomy:

- **`request`**: HTTP request wide events emitted by `reqlog.Middleware()`.
  Contains method, path, status, duration, remote addr, request ID.
- **`task`**: Background task completion events. Emitted by apps after a
  unit of work finishes (e.g., CI runs, scrobble fetches). Contains
  `task.name`, `task.status`, `task.duration_ms`, plus domain-specific
  fields flattened with dotted keys.
- **`start`**: App startup events (e.g., `slog.Info("start", ...)`).
- **`fatal`**: Unrecoverable errors causing app exit
  (e.g., `slog.Error("fatal", ...)`).

## Request Tracing

Each request gets a unique ID via the `X-Request-ID` header. The proxy
forwards this header to backends, and the logs app can display all events
sharing a request ID in a trace view.

## Uptime Monitoring (`apps/monitor`)

The monitor app runs health checks every 60 seconds against:
- 13 personal domain redirects (checking for 301 + correct Location header)
- 2 content checks (monks.co homepage and piano.computer)

For each passing check, it pings Dead Man's Snitch (`https://nosnch.in/<id>`).
If the monitor process dies or checks fail, the missing heartbeat triggers
an alert.

Individual apps (e.g., `scrobbles`) also ping Dead Man's Snitch after
successful background tasks.

## Prometheus Metrics

The proxy exports Prometheus metrics on `:9999`:
- Request count (labeled by host, app, path, status code, user agent)
- Request duration histogram
