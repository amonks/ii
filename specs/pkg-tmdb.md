# TMDB Package

## Overview

Client for The Movie Database (TMDb) API v3/v4. Covers movies, TV shows,
seasons, episodes, search, lists, and credits. Includes a `MockTMDB` for
testing.

Code: [pkg/tmdb/](../pkg/tmdb/)

## API

### Client

`New(apiKey, readToken string) *Client`

### Movie Operations

- `Search(query, year string) ([]SearchResult, error)` — search movies
- `Get(id int) (*Movie, error)` — fetch movie details
- `GetCredits(id int) (*Credits, error)` — fetch cast and crew
- `List(listID int) ([]Movie, error)` — fetch TMDB list contents
- `AddToList(listID, movieID int) error` — add movie to TMDB list (v4 API)
- `AuthorizeV4WriteAPI()` — interactive OAuth flow for v4 write access

### TV Operations

- `SearchTV(query, year string) ([]TVSearchResult, error)`
- `GetTV(id int) (*TVShow, error)`
- `GetSeason(tvID, seasonNumber int) (*Season, error)`
- `GetEpisode(tvID, seasonNumber, episodeNumber int) (*Episode, error)`

### Key Types

- `Movie` — `ID`, `Title`, `OriginalTitle`, `Overview`, `Runtime`,
  `ReleaseDate`, `Genres`, `Languages`, `PosterPath`, etc.
- `TVShow` — `ID`, `Name`, `Overview`, `Status`, `FirstAirDate`,
  `LastAirDate`, `Genres`, `PosterPath`, etc.
- `Season` — `ID`, `SeasonNumber`, `Name`, `EpisodeCount`, `Episodes`
- `Episode` — `ID`, `EpisodeNumber`, `Name`, `Runtime`, `AirDate`
- `Credits` — `Cast []Person`, `Crew []Person`

### Testing

`NewMockTMDB() *MockTMDB` provides test doubles with `AddMovie`,
`AddTVShow`, `AddSeason`, `AddEpisode`, `AddSearchResults`,
`AddTVSearchResults`.

## External API

Base URL: `https://api.themoviedb.org/3` (v3) and `/4` (v4).
Poster images: `https://image.tmdb.org/t/p/original/<path>`.
