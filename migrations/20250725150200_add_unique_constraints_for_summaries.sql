-- +goose Up
-- Add unique constraints to prevent duplicate summaries
-- Only one summary per chat
ALTER TABLE chat_summaries ADD CONSTRAINT unique_chat_summary UNIQUE (chat_id);

-- Only one user summary per chat-user pair
ALTER TABLE user_summaries ADD CONSTRAINT unique_user_summary UNIQUE (chat_id, user_id);

-- +goose Down
-- Remove unique constraints
ALTER TABLE user_summaries DROP CONSTRAINT IF EXISTS unique_user_summary;
ALTER TABLE chat_summaries DROP CONSTRAINT IF EXISTS unique_chat_summary;
