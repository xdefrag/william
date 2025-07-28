-- +goose Up
-- Add updated_at columns to summary tables
ALTER TABLE chat_summaries 
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

ALTER TABLE user_summaries 
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

-- Update existing records to have updated_at = created_at
UPDATE chat_summaries SET updated_at = created_at WHERE updated_at IS NULL;
UPDATE user_summaries SET updated_at = created_at WHERE updated_at IS NULL;

-- Add unique constraint for user summaries (if it doesn't exist)
-- Drop and recreate to avoid conflicts
ALTER TABLE user_summaries 
DROP CONSTRAINT IF EXISTS unique_user_summary;

ALTER TABLE user_summaries 
ADD CONSTRAINT unique_user_summary UNIQUE (chat_id, user_id);

-- +goose Down
-- Remove unique constraints
ALTER TABLE user_summaries 
DROP CONSTRAINT IF EXISTS unique_user_summary;

-- Remove updated_at columns
ALTER TABLE chat_summaries 
DROP COLUMN IF EXISTS updated_at;

ALTER TABLE user_summaries 
DROP COLUMN IF EXISTS updated_at;
