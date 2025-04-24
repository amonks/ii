-- Add tv_results column to stubs table if it doesn't exist
ALTER TABLE stubs ADD COLUMN tv_results text;