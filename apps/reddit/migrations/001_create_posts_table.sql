CREATE TABLE IF NOT EXISTS posts (
	name         text primary key not null,
	title        text not null,
	author       text not null,
	subreddit    text not null,
	url          text not null,
	permalink    text not null,
	json         text not null,
	status       text not null default 'initial',
	filetype     text,
	archivepath  text,
	created      datetime,
	is_gallery   BOOLEAN DEFAULT FALSE,
	gallery_size INTEGER DEFAULT 0
);