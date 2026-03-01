# Reddit

## Overview

Personal Reddit saved-posts archive and browser. Authenticates with Reddit's
OAuth API to fetch saved posts, downloads media to local disk, stores
metadata in SQLite, and serves a browseable web UI.

Code: [apps/reddit/](../apps/reddit/)

## Modes

Two CLI modes:

- **Server mode** (default): serves the web UI on the tailnet.
- **Update mode** (`./reddit update`): fetches new saved posts from the
  Reddit API, then downloads pending media. Prints interactive OAuth auth
  instructions if tokens are missing.

## Data Model

### posts

| Column | Type | Notes |
|--------|------|-------|
| `name` | text PK | Reddit post name (e.g., `t3_abc123`) |
| `title` | text | |
| `author` | text | Reddit username |
| `subreddit` | text | |
| `url` | text | Original media URL |
| `permalink` | text | |
| `json` | text | Full raw API JSON blob |
| `status` | text | `new`, `archived`, `deleted`, `unsupported` |
| `filetype` | text | e.g., `.jpg`, `.mp4` |
| `archivepath` | text | Absolute path to downloaded file |
| `created` | datetime | Post creation time |
| `is_gallery` | bool | True for Reddit gallery posts |
| `gallery_size` | int | Number of gallery items |
| `is_starred` | bool | User-toggled favorite |

## Archive Pipeline

### Phase 1: UpdateArchive

Paginates through Reddit's `/user/{username}/saved` endpoint (100 posts/page).
New posts are stored with `status = "new"`. Rate limits are tracked via
`X-Ratelimit-*` headers.

### Phase 2: ProcessUnarchived

Downloads media for all `status = "new"` posts:
- Standard image/video URLs: downloaded directly.
- Gallery posts: each item downloaded individually using media IDs from
  the post's JSON. Expired URLs trigger a fresh API fetch.
- RedGifs URLs: fetches anonymous token from RedGifs API, then downloads
  the HD MP4.
- Gone media (404): marked `status = "deleted"`.
- Unsupported formats: marked `status = "unsupported"`.

Media files stored at `/data/tank/mirror/reddit/`. Gallery items named
`t3_abc123-1.jpg`, `t3_abc123-2.jpg`, etc.

## OAuth

Standard Reddit OAuth2 with permanent refresh token stored at
`/data/tank/mirror/reddit/.tokens.json`. On first run, the user completes
a manual browser-based authorization flow.

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Grid/list of archived posts with filter support |
| GET | `/post/{n}/` | Single post viewer with wrap-around pagination |
| GET | `/subreddits/` | All subreddits with post counts |
| GET | `/authors/` | All authors with post counts |
| POST | `/star/{name}/` | Toggle starred status |
| GET | `/media/` | Static file server for downloaded media |

Filter query params: `subreddit`, `author`, `starred`.

## Deployment

Runs on **thor** (local server). Database at
`/data/tank/mirror/reddit/.reddit.db`. Private access tier.
