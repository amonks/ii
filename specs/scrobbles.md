# Scrobbles

## Overview

Personal music listening history viewer. Polls the Last.fm API for recent
scrobbles and stores them in SQLite, then serves a simple web table.

Code: [apps/scrobbles/](../apps/scrobbles/)

## Data Model

### artists

`url` (PK), `name`, `mbid`

### albums

Composite PK `(artist_url, name)`, `mbid`

### tracks

`url` (PK), `name`, `mbid`, `artist_url`, `album_name`

### scrobbles

Composite PK `(date, track_url)`

Deduplication: duplicate scrobbles return `ErrDuplicate` (silently skipped).
Currently-playing tracks (no timestamp) return `ErrStillListening` (also
skipped).

## Background Fetch

A goroutine calls `pkg/lastfm` every hour to fetch the latest scrobbles
for user `andrewmonks`. Uses `user.getrecenttracks` with automatic
pagination and exponential-backoff retry (up to 5 retries). Requires env var
`LASTFM_API_KEY`.

After each successful fetch, pings Dead Man's Snitch
(`https://nosnch.in/537206854d`).

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Table of the 1000 most recent scrobbles: date, track, artist, album |

## Deployment

Runs on **brigid** (local server). Database at `$MONKS_DATA/scrobbles.db`.
Private access tier.
