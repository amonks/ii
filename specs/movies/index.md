# Movies

## Overview

Personal media library manager for movies and TV shows. Handles the full
pipeline from downloading torrents on a remote seedbox to organizing files
on a local NAS, enriching with metadata, tracking watch history from
Letterboxd, and serving a private web UI for browsing and playback.

Code: [apps/movies/](../../apps/movies/)

## Specs

### [Pipeline](pipeline.md)

The import pipeline: how files are discovered in the seedbox mount,
identified via LLM + TMDB search, confirmed by the operator, enriched
with metadata (TMDB, Metacritic, posters, credits), and copied to the
organized library. The stub system, background workers, and pub/sub
notification pattern.

### [Database](database.md)

The SQLite schema: movies, TV shows/seasons/episodes, watches, stubs,
ignores, and the watch queue. FTS4 tables for search. The pub/sub
notification channels.

### [Web UI](web-ui.md)

Routes, handlers, templates, and the HTMX-driven interface. Movie and
TV browsing, filtering, import management, Metacritic validation, and
VLC playback integration.

## Architecture

The app is a single long-running binary that launches all subsystems as
goroutines coordinated via `pkg/loggingwaitgroup`. A pub/sub pattern on
the DB triggers downstream tasks when upstream tasks write new data.

### Sub-packages

| Package | Purpose |
|---------|---------|
| `config` | Constants: DB path, import/library dirs |
| `db` | SQLite models, pub/sub channels, schema/migrations |
| `filenames` | Regex-based season/episode extraction from filenames |
| `libraryserver` | HTTP handlers and templ templates |
| `letterboxdimporter` | Letterboxd RSS → watch records |
| `movieimporter` | Walk movie download dir → stubs |
| `tvimporter` | Walk TV download dir → stubs |
| `moviecopier` | Copy movie files from import to library |
| `tvcopier` | Copy TV episodes, create directory structure |
| `moviemetadatafetcher` | Fetch full TMDB JSON |
| `tvmetadatafetcher` | Fetch TV show posters |
| `creditsfetcher` | Fetch TMDB credits, extract director/writer |
| `posterfetcher` | Download poster images |
| `ratingfetcher` | Scrape Metacritic ratings |
| `stubquerygenerator` | LLM filename parsing + TMDB search |
| `moviesyncer` | (Disabled) Sync library to TMDB list |

### CLI Commands

| Command | Purpose |
|---------|---------|
| `cmd/copyprogress` | Report copy progress stats |
| `cmd/movieverifier` | Verify file existence and size match |
| `cmd/moviemissingsource` | Report movies whose source is missing |
| `cmd/tvpathfixer` | Fix library paths when naming changes |

## Deployment

Runs on **thor** (local server). Tailnet members access tier.
Database at `/data/tank/movies/.movies.db`.
