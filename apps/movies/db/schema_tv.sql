-- TV shows table
CREATE TABLE IF NOT EXISTS tv_shows (
    id                 int primary key,
    name               text,
    original_name      text,
    overview           text,
    status             text,
    first_air_date     text,
    last_air_date      text,
    genres             text,
    languages          text,
    tmdb_json          text,
    poster_path        text,
    tmdb_credits_json  text,
    imported_at        text,
    library_path       text unique
);

-- Seasons table
CREATE TABLE IF NOT EXISTS tv_seasons (
    id                 int,
    show_id            int,
    season_number      int,
    name               text,
    overview           text,
    episode_count      int,
    air_date           text,
    poster_path        text,
    imported_at        text,
    tmdb_json          text,
    library_path       text unique,
    PRIMARY KEY (show_id, season_number),
    FOREIGN KEY (show_id) REFERENCES tv_shows(id)
);

-- Episodes table
CREATE TABLE IF NOT EXISTS tv_episodes (
    id                 int,
    show_id            int,
    season_number      int,
    episode_number     int,
    name               text,
    overview           text,
    runtime            int,
    air_date           text,
    still_path         text,
    extension          text,
    imported_from_path text unique,
    library_path       text unique,
    is_copied          boolean not null default false,
    imported_at        text,
    tmdb_json          text,
    PRIMARY KEY (show_id, season_number, episode_number),
    FOREIGN KEY (show_id, season_number) REFERENCES tv_seasons(show_id, season_number)
);

-- Add FTS table for TV show search
CREATE VIRTUAL TABLE IF NOT EXISTS tv_show_titles using fts4(
    id,
    name
);