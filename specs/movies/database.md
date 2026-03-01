# Movies: Database

## Overview

SQLite database at `/data/tank/movies/.movies.db`, opened with WAL mode via
GORM. Schema is managed by embedded SQL migration strings.

## Tables

### movies

| Column | Type | Notes |
|--------|------|-------|
| `id` | integer PK | TMDB ID |
| `title` | text | |
| `original_title` | text | |
| `tagline` | text | |
| `overview` | text | |
| `runtime` | integer | Minutes |
| `genres` | text | JSON array |
| `languages` | text | JSON array |
| `release_date` | text | |
| `extension` | text | File extension |
| `library_path` | text UNIQUE | Path in organized library |
| `imported_from_path` | text UNIQUE | Original download path |
| `tmdb_json` | text | Full TMDB response |
| `poster_path` | text | TMDB poster path |
| `tmdb_credits_json` | text | Full credits response |
| `director_name` | text | First director |
| `writer_name` | text | First writer |
| `imported_at` | datetime | |
| `is_copied` | bool | Whether file has been copied to library |
| `metacritic_rating` | integer | Critic score |
| `metacritic_url` | text | |
| `metacritic_validated` | bool | Operator-confirmed score |

### movie_titles (FTS4)

Full-text search on `(id, title)`.

### watches

| Column | Type | Notes |
|--------|------|-------|
| `letterboxd_url` | text PK | Diary entry URL |
| `date` | text | Watch date |
| `review` | text | |
| `movie_title` | text | |
| `rating` | real | Star rating |
| `movie_release_year` | integer | |
| `movie_letterboxd_url` | text | |
| `is_liked` | bool | |
| `is_rewatch` | bool | |

### watch_titles (FTS4)

Full-text search on `(letterboxd_url, title)`.

### movie_watches

Join table: `(id, letterboxd_url)` linking movies to watches.

### queued_movies

| Column | Type | Notes |
|--------|------|-------|
| `id` | integer PK | Movie TMDB ID |
| `queued_at` | datetime | |

Movies are dequeued automatically when a matching Letterboxd watch is
imported.

### ignores

| Column | Type | Notes |
|--------|------|-------|
| `path` | text PK | File/directory path |
| `type` | integer | 1=TV, 2=Movie |
| `ignore_type` | integer | 1=whole show dir, 2=specific episode |

### stubs

| Column | Type | Notes |
|--------|------|-------|
| `imported_from_path` | text PK | |
| `type` | integer | 1=TV, 2=Movie |
| `year` | integer | Parsed from filename |
| `query` | text | Search query |
| `results` | text | JSON TMDB search results |
| `tv_results` | text | JSON TV search results |
| `episode_files` | text | JSON list of file paths |
| `season_number` | integer | |
| `search_status` | text | |

### tv_shows

| Column | Type | Notes |
|--------|------|-------|
| `id` | integer PK | TMDB ID |
| `name`, `original_name` | text | |
| `overview`, `status` | text | |
| `first_air_date`, `last_air_date` | text | |
| `genres`, `languages` | text | JSON arrays |
| `tmdb_json`, `tmdb_credits_json` | text | |
| `poster_path` | text | |
| `imported_at` | datetime | |
| `library_path` | text UNIQUE | |

### tv_show_titles (FTS4)

Full-text search on `(id, name)`.

### tv_seasons

| Column | Type | Notes |
|--------|------|-------|
| `show_id` | integer | Composite PK with season_number |
| `season_number` | integer | |
| `id` | integer | TMDB season ID |
| `name`, `overview` | text | |
| `episode_count` | integer | |
| `air_date` | text | |
| `poster_path` | text | |
| `imported_at` | datetime | |
| `tmdb_json` | text | |
| `library_path` | text UNIQUE | |

### tv_episodes

| Column | Type | Notes |
|--------|------|-------|
| `show_id` | integer | Composite PK with season_number + episode_number |
| `season_number` | integer | |
| `episode_number` | integer | |
| `id` | integer | TMDB episode ID |
| `name`, `overview` | text | |
| `runtime` | integer | |
| `air_date` | text | |
| `still_path` | text | |
| `extension` | text | |
| `imported_from_path` | text UNIQUE | |
| `library_path` | text UNIQUE | |
| `is_copied` | bool | |
| `imported_at` | datetime | |
| `tmdb_json` | text | |

## Pub/Sub Channels

The DB model provides subscription channels for reactive processing:

- `Subscribe()` — notifies when movies change
- `SubscribeTV()` — notifies when TV seasons change
- `SubscribeMovieStub()` — notifies when new movie stubs appear
- `SubscribeTVStub()` — notifies when new TV stubs appear

Each returns a channel that receives one value then is replaced.
