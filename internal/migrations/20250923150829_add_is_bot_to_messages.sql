-- +goose Up
-- Add is_bot field to distinguish bot messages from user messages

-- Add is_bot column with default false for existing records
ALTER TABLE messages
ADD COLUMN is_bot BOOLEAN NOT NULL DEFAULT false;

-- Add index for efficient querying
CREATE INDEX idx_messages_chat_is_bot ON messages(chat_id, is_bot);

-- +goose Down
-- Remove the is_bot field

DROP INDEX IF EXISTS idx_messages_chat_is_bot;
ALTER TABLE messages DROP COLUMN IF EXISTS is_bot;