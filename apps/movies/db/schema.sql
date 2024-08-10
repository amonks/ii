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
	`type` integer
);
CREATE TABLE IF NOT EXISTS `stubs` (
	`imported_from_path` text,
	`year` text,
	`query` text,
	`results` text,
	`type` integer,
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
	id primary key references movies.id,
	title text
);
CREATE VIRTUAL TABLE IF NOT EXISTS watch_titles using fts4(
	letterboxd_url text primary key references watches.letterboxd_url,
	title text
);
INSERT OR IGNORE INTO movie_titles SELECT id, title FROM MOVIES;
INSERT OR IGNORE INTO watch_titles SELECT letterboxd_url, movie_title as title FROM WATCHES;
