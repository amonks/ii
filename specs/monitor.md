# Monitor

## Overview

Background service that periodically checks whether personal websites are
up and correctly configured, reporting status to Dead Man's Snitch.

Code: [apps/monitor/](../apps/monitor/)

## Check Types

### Redirect Checks

Validates HTTP 301 status and exact `Location` header value. 13 domains
are monitored, all redirecting to `monks.co/` or `belgianman.bandcamp.com/`:

belgianman.com, blgn.mn, andrewmonks.org, lyrics.gy, fuckedcars.com,
popefucker.com, docrimes.com, fmail.email, andrewmonks.net,
needsyourhelp.org, andrewmonks.com, amonks.co, ss.cx

### Body Checks

Validates that the response body contains a literal string:
- `monks.co` must contain `"I watch movies most days."`
- `piano.computer` must contain `"6 pianists"`

## Check Mechanics

`HTTPMonitor` makes GET requests with redirects disabled and a
`monks-co-monitor` User-Agent. Composable check functions:
- `WithRedirectCheck(target)` — status 301 + Location header
- `WithBodyCheck(checks...)` — reads body with `TeeReader` for multiple checks
- `LiteralCheck(strings...)` — wraps strings as regex via `QuoteMeta`
- `RegexpCheck(regexps...)` — full regex body matching

## Reporting

`Reporter` iterates all monitors every 60 seconds. For each passing check,
pings `https://nosnch.in/<snitch-id>` via `pkg/snitch`. Failures are logged
but don't stop other checks.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | 301 redirect to the Dead Man's Snitch dashboard |

## Deployment

Long-running service. Two concurrent goroutines: the check loop and a
minimal HTTP server.
