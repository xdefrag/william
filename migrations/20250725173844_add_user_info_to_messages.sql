-- +goose Up
-- Add user identification fields to messages table
ALTER TABLE messages 
ADD COLUMN user_first_name TEXT NOT NULL DEFAULT '',
ADD COLUMN user_last_name TEXT,
ADD COLUMN username TEXT;

-- Update default constraint for user_first_name to be NOT NULL without default after data migration
-- This assumes existing data will be backfilled if needed

-- +goose Down
-- Remove user identification fields
ALTER TABLE messages 
DROP COLUMN IF EXISTS user_first_name,
DROP COLUMN IF EXISTS user_last_name,
DROP COLUMN IF EXISTS username;
