# Golink

## Overview

URL shortener / "go links" service for the tailnet. Users create named
shortcuts that redirect to arbitrary URLs, managed through a simple web UI.

Code: [apps/golink/](../apps/golink/)

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | List all shortlinks |
| POST | `/` | Create/update a shortlink (form: `key`, `url`) |
| DELETE | `/<key>` | Delete a shortlink (via htmx) |
| GET | `/<key>` | Look up key, 301 redirect to target URL; 404 if not found |

The UI uses htmx for in-place deletion without page reload. Creation is a
plain HTML form POST.

## Data Model

### shortenings

| Column | Type | Notes |
|--------|------|-------|
| `key` | text PK | Short link identifier |
| `url` | text | Target URL |
| `created_at` | datetime | |

CRUD: `List`, `Get(key)`, `Set(key, url)` (upsert), `Delete(key)`.

## Base Path

Uses `serve.BasePath(req)` for the `<base href>` so links work correctly
when accessed through the proxy at a non-root path (e.g.,
`https://monks.co/golink/`).

## Deployment

Runs on **brigid** (local server). Database at `$MONKS_DATA/golink.db`.
Private access tier.
