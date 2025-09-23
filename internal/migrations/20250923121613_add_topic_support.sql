-- +goose Up
-- Add topic_id to messages table
ALTER TABLE messages
ADD COLUMN topic_id BIGINT;

-- Add topic_id to chat_summaries table
ALTER TABLE chat_summaries
ADD COLUMN topic_id BIGINT;

-- Remove old unique constraint on chat_id only
ALTER TABLE chat_summaries
DROP CONSTRAINT IF EXISTS unique_chat_summary;

-- Add new unique constraint for (chat_id, topic_id) in chat_summaries
ALTER TABLE chat_summaries
ADD CONSTRAINT unique_chat_topic UNIQUE (chat_id, topic_id);

-- Indexes for efficient querying
-- Index for messages by chat and topic
CREATE INDEX idx_messages_chat_topic_id_desc
ON messages(chat_id, topic_id, id DESC);

-- Index for chat summaries by chat and topic
CREATE INDEX idx_chat_summaries_chat_topic_created_desc
ON chat_summaries(chat_id, topic_id, created_at DESC);

-- +goose Down
-- Remove indexes
DROP INDEX IF EXISTS idx_chat_summaries_chat_topic_created_desc;
DROP INDEX IF EXISTS idx_messages_chat_topic_id_desc;

-- Remove new unique constraint
ALTER TABLE chat_summaries
DROP CONSTRAINT IF EXISTS unique_chat_topic;

-- Restore old unique constraint on chat_id only
ALTER TABLE chat_summaries
ADD CONSTRAINT unique_chat_summary UNIQUE (chat_id);

-- Remove columns
ALTER TABLE chat_summaries
DROP COLUMN IF EXISTS topic_id;

ALTER TABLE messages
DROP COLUMN IF EXISTS topic_id;