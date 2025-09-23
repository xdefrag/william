-- +goose Up
-- Fix constraints that are causing conflicts

-- Drop the old unique_chat_summary constraint (force removal)
ALTER TABLE chat_summaries DROP CONSTRAINT IF EXISTS unique_chat_summary;

-- Ensure the new constraint exists
ALTER TABLE chat_summaries DROP CONSTRAINT IF EXISTS unique_chat_topic;
ALTER TABLE chat_summaries ADD CONSTRAINT unique_chat_topic UNIQUE (chat_id, topic_id);

-- +goose Down
-- Restore original constraint
ALTER TABLE chat_summaries DROP CONSTRAINT IF EXISTS unique_chat_topic;
ALTER TABLE chat_summaries ADD CONSTRAINT unique_chat_summary UNIQUE (chat_id);