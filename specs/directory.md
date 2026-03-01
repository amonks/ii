# Directory

## Overview

Internal service directory that renders a matrix table showing which apps
are running on which machines across the tailnet. Reads per-machine TOML
config files at startup to build the table.

Code: [apps/directory/](../apps/directory/)

## Data Loading

`LoadTable()` at startup:
1. Reads all `*.toml` files (excluding `fly-apps.toml`) from
   `$MONKS_ROOT/config/`.
2. For each machine, calls `config.Load(machine)` to parse the TOML and
   resolve variables.
3. Unions all known app names across all machines.
4. For each app + machine pair, if hosted, sets a cell value of
   `http://monks-<app>-<machine>/`.

Returns a `Table{Headers, Rows}` struct for the template.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Renders the directory table via `templ`. URLs are clickable links. |

## Data Storage

None. Reads config files at startup; all data is in memory.

## Deployment

Runs on **brigid** (local server). Private access tier.
