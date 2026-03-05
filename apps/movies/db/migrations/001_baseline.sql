CREATE TABLE IF NOT EXISTS movies (
	id                 int primary key,
	title              text,
	original_title     text,
	tagline            text,
	overview           text,
	runtime            int,
	genres             text,
	languages          text,
	release_date       text,

	extension          text,
	library_path       text unique,
	imported_from_path text unique,
	tmdb_json text,
	poster_path text,
	tmdb_credits_json text,
	director_name text,
	writer_name text,
	imported_at text,
	is_copied boolean not null default false,
	`metacritic_rating` integer,
	`metacritic_url` text,
	`metacritic_validated` numeric
);
CREATE TABLE IF NOT EXISTS ignores (
	path text unique,
	`type` integer,
	`ignore_type` integer default 1
);
CREATE TABLE IF NOT EXISTS `stubs` (
	`imported_from_path` text,
	`year` text,
	`query` text,
	`results` text,
	`tv_results` text,
	`type` integer,
	`episode_files` text,
	`season_number` INTEGER DEFAULT 1,
	`search_status` TEXT DEFAULT 'pending',
	PRIMARY KEY (`imported_from_path`)
);
CREATE TABLE IF NOT EXISTS `watches` (
	`date` datetime,
	`review` text,
	`movie_title` text,
	`rating` integer,
	`letterboxd_url` text,
	`movie_release_year` integer,
	`movie_letterboxd_url` text,
	`is_liked` numeric,
	`is_rewatch` numeric,
	PRIMARY KEY (`letterboxd_url`)
);
CREATE TABLE IF NOT EXISTS "queued_movies" (
	id primary key,
	queued_at text
);
CREATE VIRTUAL TABLE IF NOT EXISTS movie_titles using fts4(
	id,
	title
);
CREATE VIRTUAL TABLE IF NOT EXISTS watch_titles using fts4(
	letterboxd_url,
	title
);

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
