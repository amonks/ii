# Movies: Import Pipeline

## Overview

The import pipeline moves media from the seedbox mount to the organized
NAS library through a multi-stage process involving file discovery, LLM
identification, operator confirmation, metadata enrichment, and file
copying.

## Filesystem Paths

| Purpose | Path |
|---------|------|
| Movie import (seedbox mount) | `/usr/home/ajm/mnt/whatbox/files/movies` |
| Movie library (NAS) | `/data/tank/movies` |
| Movie posters | `/data/tank/movies/posters/` |
| TV import (seedbox mount) | `/usr/home/ajm/mnt/whatbox/files/tv` |
| TV library (NAS) | `/data/tank/tv` |
| TV posters | `/data/tank/tv/posters/show_<id>.jpg` |

## Library Path Naming

Movies: `<YYYY>-<Title><.ext>` with illegal characters replaced by `-`.
Example: `2011-The-Descendants.mkv`

TV: `<Show Name (YYYY)>/Season <N>/S<NN>E<NN> - <Episode Name>.<ext>`

## Stub Pipeline

A **stub** is a record for a file found in the import directory but not
yet identified. The pipeline has four stages:

### 1. Discovery (movieimporter / tvimporter)

- Movie importer runs every 1 minute, walks the movie download dir, creates
  a stub for each unrecognized `.mkv` file.
- TV importer runs every 1 minute, walks the TV download dir by show
  directory, detects episode files using regex patterns in the `filenames`
  package, creates/updates stubs.

### 2. LLM Query Generation (stubquerygenerator)

Triggered reactively via `db.SubscribeMovieStub()` / `db.SubscribeTVStub()`.
Calls the `llm` CLI (`llm -m 4o-mini --schema ...`) to parse the filename
into a structured `{title, year}` search query. Then immediately searches
TMDB and stores the results in the stub.

### 3. Operator Confirmation (libraryserver /import/)

The operator visits the import page, sees stubs with pre-filled TMDB search
results, and either:
- **Identifies** the stub: selects the correct TMDB match, which creates a
  Movie or TVEpisode record and deletes the stub.
- **Ignores** the file: creates an ignore record so the importer skips it.

The operator can also manually search TMDB if the auto-results are wrong.

### 4. Metadata Enrichment (reactive workers)

Once a Movie or TVEpisode record exists, reactive workers are triggered via
pub/sub:

| Worker | Trigger | Action |
|--------|---------|--------|
| `creditsfetcher` | movie change | Fetch TMDB credits, extract first director and writer |
| `moviemetadatafetcher` | movie change | Fetch full TMDB JSON |
| `posterfetcher` | movie change | Download poster image to disk |
| `ratingfetcher` | movie change | Scrape Metacritic rating (1s delay between requests) |
| `moviecopier` | movie change | Copy file from import dir to library dir with progress logging |
| `tvcopier` | TV season change | Copy episode files, create directory structure |
| `tvmetadatafetcher` | TV season change | Download TV show poster |

## Pub/Sub Pattern

Each `Subscribe*()` call on the DB returns a channel. Workers loop:
run → wait for next notification or context cancel → run again. The channel
is closed and replaced after each notification, so workers process all
pending items on each trigger.

## Metacritic Validation

Scraped Metacritic results are saved with `metacritic_validated = false`.
The import page shows unvalidated movies for operator confirmation. The
operator verifies/corrects the URL and score, then confirms to set
`metacritic_validated = true`.

## Letterboxd Watch Import

Runs every 30 minutes. Fetches the owner's Letterboxd RSS diary, creates
Watch records, and automatically dequeues watched movies from the queue.

## External Dependencies

- **TMDB API**: search, metadata, credits, posters
- **Metacritic**: web scraping for critic scores
- **LLM CLI** (`llm` binary): filename → search query parsing
- **Letterboxd**: RSS feed for watch history
- **Whatbox seedbox**: source of downloaded torrents (SSHFS mount)
