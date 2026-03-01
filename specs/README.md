# monks.co

Multi-app monorepo for monks.co services. Each app lives in `./apps/<name>`.

## Architecture

Apps run on different hosts but share a tailnet. The proxy app (`apps/proxy`) handles all inbound requests and forwards them to the appropriate backend app based on path.

Example: `https://monks.co/dogs` is handled by the proxy and forwarded to the dogs app.

## Hosts

- **fly** (fly.io, Chicago ORD): dogs, homepage, map, writing, traffic, logs
- **brigid** (local server): sms, calendar, directory, golink, ping, scrobbles, youtube
- **thor** (local server): air, movies, reddit

## Deployment

Deploy apps by running from the repo root:

```
fly deploy -c apps/$app/fly.toml
```

## Routing

Routing is configured through tailscale capability grants. Access is tiered:

**Public** (the whole internet):
- dogs, homepage, map, writing

**Tailnet members** (autogroup:member):
- air, movies

**Services** (tag:service):
- traffic, logs, sms

**Private** (ajm@passkey only):
- calendar, directory, golink, ping, scrobbles, logs, youtube, reddit

The full routing configuration is maintained in tailscale ACL policy as capability grants on `tag:monks-co` nodes using the `monks.co/cap/public` capability. Backend names follow the pattern `monks-<app>-<host>[-<region>]`.
