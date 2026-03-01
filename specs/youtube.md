# YouTube

## Overview

Personal YouTube watch history viewer. Reads a Google Takeout JSON export of
watch history and displays it as a simple HTML list.

Code: [apps/youtube/](../apps/youtube/)

## Data Loading

At startup, loads all JSON files from the `histories/` directory (relative to
the working directory). Each file is a Google Takeout `watch-history.json`
array. Only `Title`, `Time`, and `TitleUrl` fields are used; other fields
are ignored.

All entries are held in memory for the lifetime of the process. No hot
reload.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Full list of all watch history entries |

Each entry renders as a list item with the title (linked to the video URL
if available) and timestamp.

No sorting, filtering, pagination, or search.

## Data Storage

Flat JSON files on disk in `histories/`. Currently one file:
`histories/ajm@andrewmonks.net.json`.

## Deployment

Runs on **brigid** (local server). Private access tier.
