# Specifications

## Architecture

### Monorepo

This is a multi-project monorepo. Web apps live in ./apps, libraries live in ./pkg, and command line tools live in ./cmd. Each one is a go module. They should share infrastructure to the extent possible.

We use Jujutsu for version control. NEVER USE GIT COMMANDS, either at the command line on in code.

### Networking

Apps run on different hosts (either our own metal or on fly.io) and all share a tailnet. Only the proxy app may listen on a public interface. The proxy app handles all inbound requests and routes them to apps. For example, `https://monks.co/dogs/index.html` is proxied to http://monks-dogs-fly-ord/index.html.

Taliscale machine names are monks-$appname-$host, eg monks-logs-fly-ord or monks-movies-thor. Within the tailnet, they can be reached at http://$machine-name.

ALL INTERNAL TRAFFIC MUST USE THE TAILNET

### Routing

Routing is configured through tailscale capability grants. Services, apps, users, and the public can each be served different routes. In practice, though, the convention is, if you have permission to access <appname>, you can reach it at https://monks.co/<appname>.

### Operating

We use `go tool run` as a task runner. It runs dags of bash scripts defined in tasks.toml files. Eg, `go tool run test` looks at ./tasks.toml for a task called 'test' and runs it after running its dependencies.

### Testing

The test command is `go tool run test`. It runs go test monks.co/..., a variety of static analysis tools, and any task 'test' defined in any module's taskfile.

ALWAYS use `go tool run test` after making changes to make sure everything's passing.

For changes to CI infrastructure (builder.Dockerfile, builder code, CI config), local tests are not sufficient. You must push the change and verify the CI run succeeds. See the "Builder Image" section of [ci.md](ci.md) for the manual push workflow. The CI dashboard is at http://monks-ci-fly-ord/ on the tailnet.

### Deployment

Our CI/CD system does three kinds of deployment:

- "publishing" (a small subset of) modules to github as public read-only mirrors
- "deploying" (a small subset of) apps to fly.io. App names are 'monks-$appname', eg monks-logs or monks-proxy.
- "terraform" (see ./aws)

## Specs

Each area of the system should have a spec. Keep specs up to date while working on the system. Study relvant specs before beginning work. Every spec must be included in this index, along with a description sufficiently detailed as to inform a reader whether the spec is relevant to their task.

| Spec                                  | Purpose                                                                                                   |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| [tailnet-routing](tailnet-routing.md) | How apps join the tailnet, capability-based routing, access tiers, the proxy's dual-listener architecture |
| [observability](observability.md)     | The reqlog → logsclient → logs pipeline, request tracing, uptime monitoring, Prometheus metrics           |
| [app-boilerplate](app-boilerplate.md) | Standard app startup pattern, shared packages, environment variables, deployment, templating              |
| [publish](publish.md)                 | Publishing monorepo subtrees as public GitHub mirrors, validation, vanity imports                         |
| [deploy](deploy.md)                   | Automated Fly.io deployment with change detection and dependency graph                                    |
| [terraform](terraform.md)             | AWS infrastructure: DNS for 15 domains, SES email, CloudFront CDN, IAM, remote state                      |

## Apps

| Spec                              | Code                                    | Purpose                                                              |
| --------------------------------- | --------------------------------------- | -------------------------------------------------------------------- |
| [air](air.md)                     | [apps/air/](../apps/air/)               | Home environmental monitoring dashboard (CO2, temperature, humidity) |
| [aranet](aranet.md)               | [apps/aranet/](../apps/aranet/)         | Bluetooth scanning service for Aranet4 sensors                       |
| [calendar](calendar.md)           | [apps/calendar/](../apps/calendar/)     | Personal TV show tracking calendar                                   |
| [ci](ci.md)                       | [apps/ci/](../apps/ci/)                 | Self-hosted CI/CD: orchestrator + ephemeral builder, builder image config |
| [creamery](creamery.md)           | [apps/creamery/](../apps/creamery/)     | Ice cream formulation and batch log analytics                        |
| [directory](directory.md)         | [apps/directory/](../apps/directory/)   | Internal service directory showing apps × machines                   |
| [dogs](dogs.md)                   | [apps/dogs/](../apps/dogs/)             | Hot dog eating contest photo gallery                                 |
| [dolmenwood](dolmenwood/index.md) | [apps/dolmenwood/](../apps/dolmenwood/) | Dolmenwood RPG character sheet manager                               |
| [dungeon](dungeon.md)             | [apps/dungeon/](../apps/dungeon/)       | Player-facing TTRPG mapping tool                                     |
| [golink](golink.md)               | [apps/golink/](../apps/golink/)         | URL shortener / go links for the tailnet                             |
| [homepage](homepage.md)           | [apps/homepage/](../apps/homepage/)     | Personal homepage with Letterboxd integration                        |
| [logs](logs.md)                   | [apps/logs/](../apps/logs/)             | Centralized observability dashboard                                  |
| [mailer](mailer.md)               | [apps/mailer/](../apps/mailer/)         | Internal email-sending service (AWS SES)                             |
| [map](map.md)                     | [apps/map/](../apps/map/)               | Interactive Google Map of visited places                             |
| [monitor](monitor.md)             | [apps/monitor/](../apps/monitor/)       | Uptime monitoring with Dead Man's Snitch                             |
| [movies](movies/index.md)         | [apps/movies/](../apps/movies/)         | Movie and TV show library manager                                    |
| [ping](ping.md)                   | [apps/ping/](../apps/ping/)             | Personal relationship / keep-in-touch tracker                        |
| [proxy](proxy.md)                 | [apps/proxy/](../apps/proxy/)           | TLS-terminating reverse proxy for monks.co                           |
| [reddit](reddit.md)               | [apps/reddit/](../apps/reddit/)         | Reddit saved-posts archive and browser                               |
| [scrobbles](scrobbles.md)         | [apps/scrobbles/](../apps/scrobbles/)   | Music listening history viewer (Last.fm)                             |
| [sms](sms.md)                     | [apps/sms/](../apps/sms/)               | SMS sending service (Twilio)                                         |
| [vault](vault.md)                 | [apps/vault/](../apps/vault/)           | Litestream SFTP replica target on ZFS                                |
| [writing](writing.md)             | [apps/writing/](../apps/writing/)       | Personal blog with responsive images                                 |
| [youtube](youtube.md)             | [apps/youtube/](../apps/youtube/)       | YouTube watch history viewer                                         |

## Packages

### Core Infrastructure

| Spec                            | Code                              | Purpose                                                               |
| ------------------------------- | --------------------------------- | --------------------------------------------------------------------- |
| [pkg-tailnet](pkg-tailnet.md)   | [pkg/tailnet/](../pkg/tailnet/)   | tsnet wrapper: tailnet membership, HTTP client, peer identification   |
| [pkg-reqlog](pkg-reqlog.md)     | [pkg/reqlog/](../pkg/reqlog/)     | Wide-event structured HTTP request logging and log shipping           |
| [pkg-database](pkg-database.md) | [pkg/database/](../pkg/database/) | GORM + SQLite wrapper with WAL mode and migrations                    |
| [pkg-migrate](pkg-migrate.md)   | [pkg/migrate/](../pkg/migrate/)   | SQLite migration runner with version tracking and drift detection      |
| [pkg-config](pkg-config.md)     | [pkg/config/](../pkg/config/)     | Unified app config and proxy config loading from TOML                 |
| [pkg-serve](pkg-serve.md)       | [pkg/serve/](../pkg/serve/)       | HTTP mux with base-path awareness, error helpers, JSON encoding       |
| [pkg-tls](pkg-tls.md)           | [pkg/tls/](../pkg/tls/)           | ACME TLS certificate management via CertMagic                         |
| [pkg-logs](pkg-logs.md)         | [pkg/logs/](../pkg/logs/)         | SQLite log storage and adaptive query engine                          |
| [pkg-depgraph](pkg-depgraph.md) | [pkg/depgraph/](../pkg/depgraph/) | Dependency graph builder for monks.co/\* modules                      |
| [pkg-ci](pkg-ci.md)             | [pkg/ci/](../pkg/ci/)             | CI shared libraries: change detection, publish config/validation      |
| [pkg-oci](pkg-oci.md)           | [pkg/oci/](../pkg/oci/)           | Pure Go OCI image building and registry push via go-containerregistry |
| [pkg-flyapi](pkg-flyapi.md)     | [pkg/flyapi/](../pkg/flyapi/)     | Fly Machines API REST client                                          |
| —                               | [pkg/tailscaleacl/](../pkg/tailscaleacl/) | Generate Tailscale ACL JSON from apps.toml + base JSONC       |

### Domain-Specific

| Spec                      | Code                        | Purpose                                               |
| ------------------------- | --------------------------- | ----------------------------------------------------- |
| [pkg-tmdb](pkg-tmdb.md)   | [pkg/tmdb/](../pkg/tmdb/)   | TMDB API client for movies, TV, search, credits       |
| [pkg-posts](pkg-posts.md) | [pkg/posts/](../pkg/posts/) | Markdown post loading with responsive image rendering |

### All Packages

| Spec                              | Purpose                                                                                                             |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| [pkg-utilities](pkg-utilities.md) | Reference for all utility packages: HTTP, external APIs, filesystem, concurrency, config, logging, templating, auth |
