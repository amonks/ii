# Specifications

## Architecture

Multi-app monorepo for monks.co services. Each app lives in `./apps/<name>`, and shared packages live in `./pkg/<name>`.

Apps run on different hosts but share a tailnet. The proxy app (`apps/proxy`) handles all inbound requests and forwards them to the appropriate backend app based on path. For example, `https://monks.co/dogs` is handled by the proxy and forwarded to the dogs app.

### Hosts

- **fly** (fly.io, Chicago ORD): dogs, homepage, map, writing, traffic, logs
- **brigid** (local server): sms, calendar, directory, dungeon, golink, ping, scrobbles, youtube
- **thor** (local server): air, movies, reddit

### Deployment

Deploy apps from the repo root:

```
fly deploy -c apps/$app/fly.toml
```

### Routing

Routing is configured through tailscale capability grants. Access is tiered:

- **Public** (autogroup:danger-all): dogs, dungeon, homepage, map, writing
- **Tailnet members** (autogroup:member): air, movies
- **Services** (tag:service): traffic, logs, sms
- **Private** (ajm@passkey): calendar, directory, golink, ping, scrobbles, logs, youtube, reddit

The full routing configuration is maintained in tailscale ACL policy as capability grants on `tag:monks-co` nodes using the `monks.co/cap/public` capability. Backend names follow the pattern `monks-<app>-<host>[-<region>]`.

## Concepts

| Spec | Purpose |
| ---- | ------- |
| [tailnet-routing](tailnet-routing.md) | How apps join the tailnet, capability-based routing, access tiers, the proxy's dual-listener architecture |
| [observability](observability.md) | The reqlog → logsclient → logs pipeline, request tracing, uptime monitoring, Prometheus metrics |
| [app-boilerplate](app-boilerplate.md) | Standard app startup pattern, shared packages, environment variables, deployment, templating |

## Apps

| Spec | Code | Purpose |
| ---- | ---- | ------- |
| [air](air.md) | [apps/air/](../apps/air/) | Home environmental monitoring dashboard (CO2, temperature, humidity) |
| [aranet](aranet.md) | [apps/aranet/](../apps/aranet/) | Bluetooth scanning service for Aranet4 sensors |
| [calendar](calendar.md) | [apps/calendar/](../apps/calendar/) | Personal TV show tracking calendar |
| [directory](directory.md) | [apps/directory/](../apps/directory/) | Internal service directory showing apps × machines |
| [dogs](dogs.md) | [apps/dogs/](../apps/dogs/) | Hot dog eating contest photo gallery |
| [dolmenwood](dolmenwood/index.md) | [apps/dolmenwood/](../apps/dolmenwood/) | Dolmenwood RPG character sheet manager |
| [dungeon](dungeon.md) | [apps/dungeon/](../apps/dungeon/) | Player-facing TTRPG mapping tool |
| [golink](golink.md) | [apps/golink/](../apps/golink/) | URL shortener / go links for the tailnet |
| [homepage](homepage.md) | [apps/homepage/](../apps/homepage/) | Personal homepage with Letterboxd integration |
| [logs](logs.md) | [apps/logs/](../apps/logs/) | Centralized observability dashboard |
| [mailer](mailer.md) | [apps/mailer/](../apps/mailer/) | Internal email-sending service (AWS SES) |
| [map](map.md) | [apps/map/](../apps/map/) | Interactive Google Map of visited places |
| [monitor](monitor.md) | [apps/monitor/](../apps/monitor/) | Uptime monitoring with Dead Man's Snitch |
| [movies](movies/index.md) | [apps/movies/](../apps/movies/) | Movie and TV show library manager |
| [ping](ping.md) | [apps/ping/](../apps/ping/) | Personal relationship / keep-in-touch tracker |
| [proxy](proxy.md) | [apps/proxy/](../apps/proxy/) | TLS-terminating reverse proxy for monks.co |
| [reddit](reddit.md) | [apps/reddit/](../apps/reddit/) | Reddit saved-posts archive and browser |
| [scrobbles](scrobbles.md) | [apps/scrobbles/](../apps/scrobbles/) | Music listening history viewer (Last.fm) |
| [sms](sms.md) | [apps/sms/](../apps/sms/) | SMS sending service (Twilio) |
| [writing](writing.md) | [apps/writing/](../apps/writing/) | Personal blog with responsive images |
| [youtube](youtube.md) | [apps/youtube/](../apps/youtube/) | YouTube watch history viewer |

## Packages

### Core Infrastructure

| Spec | Code | Purpose |
| ---- | ---- | ------- |
| [pkg-tailnet](pkg-tailnet.md) | [pkg/tailnet/](../pkg/tailnet/) | tsnet wrapper: tailnet membership, HTTP client, peer identification |
| [pkg-reqlog](pkg-reqlog.md) | [pkg/reqlog/](../pkg/reqlog/) | Wide-event structured HTTP request logging and log shipping |
| [pkg-database](pkg-database.md) | [pkg/database/](../pkg/database/) | GORM + SQLite wrapper with WAL mode and migrations |
| [pkg-config](pkg-config.md) | [pkg/config/](../pkg/config/) | Per-machine TOML configuration loading |
| [pkg-serve](pkg-serve.md) | [pkg/serve/](../pkg/serve/) | HTTP mux with base-path awareness, error helpers, JSON encoding |
| [pkg-tls](pkg-tls.md) | [pkg/tls/](../pkg/tls/) | ACME TLS certificate management via CertMagic |
| [pkg-logs](pkg-logs.md) | [pkg/logs/](../pkg/logs/) | SQLite log storage and adaptive query engine |

### Domain-Specific

| Spec | Code | Purpose |
| ---- | ---- | ------- |
| [pkg-tmdb](pkg-tmdb.md) | [pkg/tmdb/](../pkg/tmdb/) | TMDB API client for movies, TV, search, credits |
| [pkg-posts](pkg-posts.md) | [pkg/posts/](../pkg/posts/) | Markdown post loading with responsive image rendering |

### All Packages

| Spec | Purpose |
| ---- | ------- |
| [pkg-utilities](pkg-utilities.md) | Reference for all utility packages: HTTP, external APIs, filesystem, concurrency, config, logging, templating, auth |
