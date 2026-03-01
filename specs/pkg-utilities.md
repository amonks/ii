# Utility Packages

Small, focused packages that provide single-purpose functionality.

## Data & Encoding

| Package | Code | Purpose |
|---------|------|---------|
| crdt | [pkg/crdt/](../pkg/crdt/) | Generic Last-Write-Wins CRDT registers and maps |
| gormdate | [pkg/gormdate/](../pkg/gormdate/) | Date-only type (no time) with GORM/JSON/Gob support |
| set | [pkg/set/](../pkg/set/) | Thread-safe generic set backed by a map |
| ptr | [pkg/ptr/](../pkg/ptr/) | Generic pointer helper (`String(s) *string`) |

## HTTP & Middleware

| Package | Code | Purpose |
|---------|------|---------|
| gzip | [pkg/gzip/](../pkg/gzip/) | Response gzip compression middleware |
| gzipserver | [pkg/gzipserver/](../pkg/gzipserver/) | Static file server with pre-compressed file support (`.gz`, `.br`) |
| middleware | [pkg/middleware/](../pkg/middleware/) | Composable middleware interface, `WithIndexHTML`, `WithBrowsing` |
| request | [pkg/request/](../pkg/request/) | HTTP response error checking (non-2xx → error) |
| aschrome | [pkg/aschrome/](../pkg/aschrome/) | HTTP GET with Chrome-like headers and brotli/gzip decoding |

## External Service Clients

| Package | Code | Purpose |
|---------|------|---------|
| beeminder | [pkg/beeminder/](../pkg/beeminder/) | Post datapoints to the Beeminder goal-tracking API |
| googlemaps | [pkg/googlemaps/](../pkg/googlemaps/) | Google Places API: fetch place details by URL, ID, or CID |
| lastfm | [pkg/lastfm/](../pkg/lastfm/) | Last.fm API: paginated scrobble history with retry |
| letterboxd | [pkg/letterboxd/](../pkg/letterboxd/) | Letterboxd RSS diary feed with HTTP caching and gob persistence |
| metacritic | [pkg/metacritic/](../pkg/metacritic/) | Scrape Metacritic movie search results |
| snitch | [pkg/snitch/](../pkg/snitch/) | Dead Man's Snitch heartbeat pings |
| twilio | [pkg/twilio/](../pkg/twilio/) | Send SMS via Twilio API |
| email | [pkg/email/](../pkg/email/) | Send email via AWS SES SMTP with LOGIN auth |
| emailclient | [pkg/emailclient/](../pkg/emailclient/) | Send email via the internal mailer service over tailnet |
| llm | [pkg/llm/](../pkg/llm/) | Shell out to the `llm` CLI for structured JSON responses |

## Filesystem & Concurrency

| Package | Code | Purpose |
|---------|------|---------|
| filesystem | [pkg/filesystem/](../pkg/filesystem/) | FS interface abstraction with OS and in-memory mock implementations |
| flock | [pkg/flock/](../pkg/flock/) | Exclusive file-based locking via POSIX `flock(2)` |
| hardmemo | [pkg/hardmemo/](../pkg/hardmemo/) | Disk-backed memoization with TTL and file locking |
| periodically | [pkg/periodically/](../pkg/periodically/) | Run a function immediately then on a fixed ticker interval |
| rotate | [pkg/rotate/](../pkg/rotate/) | Thread-safe round-robin rotator over a slice |
| loggingwaitgroup | [pkg/loggingwaitgroup/](../pkg/loggingwaitgroup/) | Named WaitGroup that logs Add/Done calls |
| sigctx | [pkg/sigctx/](../pkg/sigctx/) | Context cancelled on OS signals (HUP/TERM/INT/QUIT) |

## Environment & Config

| Package | Code | Purpose |
|---------|------|---------|
| env | [pkg/env/](../pkg/env/) | `$MONKS_ROOT` and `$MONKS_DATA` path resolution |
| requireenv | [pkg/requireenv/](../pkg/requireenv/) | Required env var reader (panics if unset) |
| meta | [pkg/meta/](../pkg/meta/) | App name and machine name from env/filesystem |

## Logging & Metrics

| Package | Code | Purpose |
|---------|------|---------|
| logger | [pkg/logger/](../pkg/logger/) | Minimal named logger wrapping stdlib `log` |
| logsclient | [pkg/logsclient/](../pkg/logsclient/) | Buffered JSON log line shipper (io.Writer → HTTP POST) |
| color | [pkg/color/](../pkg/color/) | Deterministic string → hex color via FNV hashing |
| prometh | [pkg/prometh/](../pkg/prometh/) | Sanitize strings into valid Prometheus label names |

## Templating & UI

| Package | Code | Purpose |
|---------|------|---------|
| templib | [pkg/templib/](../pkg/templib/) | Shared templ page/card/form components with Tailwind CSS |
| util | [pkg/util/](../pkg/util/) | Bulk-load `html/template` files from embedded FS |
| image | [pkg/image/](../pkg/image/) | EXIF-aware image dimension decoding |
| fzf | [pkg/fzf/](../pkg/fzf/) | Wrapper around `fzf` CLI for interactive fuzzy selection |
| tui | [pkg/tui/](../pkg/tui/) | Stdin prompt and fzf-backed selection menu |

## Auth & Networking

| Package | Code | Purpose |
|---------|------|---------|
| auth | [pkg/auth/](../pkg/auth/) | Tailscale-based request authentication and 401 middleware |
| migrate | [pkg/migrate/](../pkg/migrate/) | SQLite migration runner with divergence detection |

## Domain-Specific

| Package | Code | Purpose |
|---------|------|---------|
| aranet4 | [pkg/aranet4/](../pkg/aranet4/) | BLE scanning for Aranet4 sensors and binary protocol parsing |
| dogs | [pkg/dogs/](../pkg/dogs/) | Data model and Google Sheets importer for the dogs app |
| markdown | [pkg/markdown/](../pkg/markdown/) | Goldmark extensions: `ampersand` (typographic &) and `imgres` (responsive images) |
