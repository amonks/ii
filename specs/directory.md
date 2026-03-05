# Directory

## Overview

Internal service directory that renders a matrix table showing which apps
are running on which machines across the tailnet. Reads `config/apps.toml`
at startup to build the table from route definitions.

Code: [apps/directory/](../apps/directory/)

## Data Loading

`LoadTable()` at startup:
1. Reads `config/apps.toml` via `config.LoadApps()`.
2. Derives unique hosts from all routes (`cfg.ListHosts()`).
3. For each app, checks which hosts have routes and derives the backend
   URL: `http://monks-<app>-<host>/` (or `http://monks-<app>-fly-<region>/`
   for Fly apps).
4. Returns a `Table{Headers, Rows}` struct for the template.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Renders the directory table via `templ`. URLs are clickable links. |

## Data Storage

None. Reads config files at startup; all data is in memory.

## Deployment

Runs on **brigid** (local server). Private access tier.
