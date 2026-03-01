# Movies: Web UI

## Overview

The movies web UI is served over the tailnet and uses HTMX + Tailwind CSS
for interactivity. Write operations require a `Tailscale-Cap-Movies-Write`
capability header injected by the proxy.

## Routes

### Movie Routes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/` | — | Movie library grid with filters |
| GET | `/import/` | Write | Import management page |
| GET | `/ignores/` | Write | Ignored paths management |
| GET | `/poster/` | — | Serve movie poster image |
| POST | `/play/` | — | Play movie via VLC on lugh |
| POST | `/enqueue/` | — | Add movie to watch queue |
| POST | `/search/` | Write | Manual TMDB search |
| POST | `/identify/` | Write | Confirm stub identification |
| POST | `/ignore/` | Write | Ignore a file path |
| POST | `/validate-metacritic/` | Write | Confirm Metacritic score |
| POST | `/delete-ignore/` | Write | Remove an ignore entry |

### TV Routes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/tv/` | — | TV show grid |
| GET | `/tv/show/` | — | Show detail page |
| GET | `/tv/season/` | — | Season detail with episode table |
| GET | `/tv/poster/` | — | Serve show poster |
| GET | `/tv/season/poster/` | — | Serve season poster |
| POST | `/tv/play/` | — | Play episode via VLC |
| POST | `/tv/search/` | Write | Manual TV search |
| POST | `/tv/identify/` | Write | Confirm TV stub identification |
| POST | `/tv/ignore-show/` | Write | Ignore entire show directory |
| POST | `/tv/ignore-episodes/` | Write | Ignore specific episodes |

## VLC Playback

The `/play/` and `/tv/play/` handlers SSH into a Mac host named `lugh` and:
1. Open VLC with an SFTP URL: `sftp://ajm@thr.ss.cx/<library-path>`
2. Activate fullscreen via AppleScript

## Filters

The movie library supports:
- **Search**: text search via FTS4
- **Year range**: min/max year slider
- **Sort**: MC Rating, My Rating, Watch Date, Release Date, Import Date,
  Name, Runtime, Shuffle, Queue Order
- **Show filter**: All, Queue, Unwatched, Watched
- **Genres**: checkbox filter

All filters update the URL client-side via JavaScript.

## Templates

| File | Components |
|------|------------|
| `page.templ` | `Page` — HTML shell with nav bar (Library, TV Shows, Import, Ignores) |
| `movies.templ` | `Movies`, `Movie` — movie grid with poster, metadata, ratings |
| `filters.templ` | `Filters` — sidebar with search/sort/filter controls |
| `tvshows.templ` | `TVShows`, `TVShow`, `TVShowDetails`, `TVSeasonDetails`, `TVFilters` |
| `import.templ` | `Import` — tabbed Movies/TV import page |
| `stubs.templ` | `Stubs`, `Stub` — stub cards with search/identify/ignore actions |
| `metacriticvalidation.templ` | `MetacriticValidations` — confirm/correct Metacritic URLs and scores |
| `ignores.templ` | `Ignores` — table of ignored paths with remove buttons |

Import and Ignores nav links are only shown when
`CanManageLibrary(ctx)` is true (write capability present).

## Watch Queue

Movies are enqueued via POST `/enqueue/`. Queue is ordered by `queued_at`.
Movies are dequeued automatically when a matching Letterboxd watch is
imported. The library view can be filtered to show only queued movies.
