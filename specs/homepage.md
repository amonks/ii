# Homepage

## Overview

Personal homepage for monks.co. Displays a biography, work history, location
history, and a live list of recently-watched movies pulled from Letterboxd.

Code: [apps/homepage/](../apps/homepage/)

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Main page: fetches Letterboxd diary, renders the homepage |
| GET | `/error/` | Debug handler: responds with any requested HTTP status code |

## Letterboxd Integration

Uses `pkg/letterboxd` to fetch the RSS diary for user `amonks`. The feed is
cached on the persistent Fly volume as a gob-encoded file with a file lock
to prevent concurrent fetches. Cache respects HTTP `Cache-Control: max-age`,
`ETag`, and `If-Modified-Since`. Default max-age is 1 hour. Falls back to
stale cache on network errors.

The 5 most recently-watched films are displayed with star ratings (½☆ through
★★★★★ on a half-star granularity scale). If the fetch fails, the movie
section is omitted.

## Go Board Game Rotation

Uses `pkg/rotate` to cycle through localized names for "Go" (the board game)
on each page load: Go, 囲碁, いご, 바둑, baduk, 圍棋, wéiqí.

## Data Storage

No database. Letterboxd RSS cached on the Fly volume as
`letterboxd-rss.gob` with `letterboxd-rss.lock`.

## Deployment

Runs on **fly.io** (Chicago ORD). Public access tier.
