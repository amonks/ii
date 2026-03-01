# Request Logging Package

## Overview

Wide-event structured HTTP request logging middleware. Accumulates attributes
throughout a request and emits a single `slog` JSON event on completion.
Also ships logs to the internal logs service over the tailnet.

Code: [pkg/reqlog/](../pkg/reqlog/)

## Setup

`SetupLogging()` configures:
1. A `slog.JSONHandler` writing to stderr.
2. A `logsclient.Client` that buffers and ships log lines to
   `http://monks-logs-fly-ord/ingest` via the tailnet HTTP client.
3. Redirects stdlib `log` output through slog.

`Shutdown()` flushes buffered logs.

## Middleware

`Middleware()` returns an HTTP middleware that:
1. Generates or reads a request ID (`X-Request-ID` header).
2. Records start time, method, host, path, remote address.
3. Wraps the `ResponseWriter` to capture the status code.
4. On response completion, emits a single log event with all accumulated
   attributes.

### Attribute Accumulation

`Set(ctx, key, value)` adds a key/value pair to the current request's wide
event from anywhere in the handler chain. This is used by the proxy to add
`proxy_upstream`, by `serve.Error` to add error details, etc.

## Exports

- `SetupLogging()`, `SetupStdLog()`, `Shutdown()`
- `Middleware() middleware.Middleware`
- `Set(ctx, key, value)` — add attribute to wide event
- `Status(ctx) int` — response status from context
- `RequestID(ctx) string`
- `Logger(ctx) *slog.Logger` — pre-populated logger with request ID
- `RemoteAddrKey` — context key for real client address
- `RequestIDHeader` — `"X-Request-ID"`

## Dependencies

- `pkg/logsclient` — buffered log shipping
- `pkg/meta` — app/machine name for log context
- `pkg/middleware` — middleware interface
- `pkg/tailnet` — HTTP client for log shipping
