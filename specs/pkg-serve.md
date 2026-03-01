# Serve Package

## Overview

HTTP server utilities: a context-aware listener, a `ServeMux` wrapper with
base-path awareness and route tracking, error response helpers, and JSON
encoding.

Code: [pkg/serve/](../pkg/serve/)

## Mux

`NewMux() *Mux` wraps `http.ServeMux` with two additions:

1. **Base path injection**: reads `X-Forwarded-Prefix` from the request
   (set by the proxy) and makes it available via `BasePath(r)` and
   `BasePathFromContext(ctx)`. Templates use this for `<base href>`.

2. **Route tracking**: sets the `x-mux-route` response header to the
   matched pattern, enabling observability in the logging pipeline.

## Serving

`ListenAndServe(ctx, addr, handler) error` — creates an `http.Server`
that shuts down gracefully when the context is cancelled.

## Error Helpers

- `Errorf(w, r, code, format, args...)` — writes an HTTP error and records
  it in the reqlog wide event.
- `Error(w, r, code, err)` — same, from an error value.
- `InternalServerErrorf`, `InternalServerError` — 500 shortcuts.

## JSON

`JSON(w, req, data)` — marshals data as JSON and writes with the correct
content type.

## Dependencies

- `pkg/reqlog` — for error attribute recording
