-- Add episode_files column to stubs table if it doesn't exist
ALTER TABLE stubs ADD COLUMN episode_files text;