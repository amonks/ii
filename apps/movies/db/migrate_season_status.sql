-- Add season_number and search_status columns to stubs table if they don't exist
ALTER TABLE stubs ADD COLUMN season_number INTEGER DEFAULT 1;
ALTER TABLE stubs ADD COLUMN search_status TEXT DEFAULT 'pending';