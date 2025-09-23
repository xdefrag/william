-- +goose Up
-- Add topic support to message counters

-- Add topic_id column
ALTER TABLE message_counters
ADD COLUMN topic_id BIGINT;

-- Remove old unique constraint on chat_id only
ALTER TABLE message_counters
DROP CONSTRAINT IF EXISTS message_counters_chat_id_key;

-- Add new unique constraint for (chat_id, topic_id)
ALTER TABLE message_counters
ADD CONSTRAINT unique_message_counter_chat_topic UNIQUE (chat_id, topic_id);

-- Update index to include topic_id
DROP INDEX IF EXISTS idx_message_counters_chat_id;
CREATE INDEX idx_message_counters_chat_topic ON message_counters(chat_id, topic_id);

-- +goose Down
-- Restore original structure

-- Remove new constraint and index
DROP INDEX IF EXISTS idx_message_counters_chat_topic;
ALTER TABLE message_counters DROP CONSTRAINT IF EXISTS unique_message_counter_chat_topic;

-- Restore old constraint and index
ALTER TABLE message_counters ADD CONSTRAINT message_counters_chat_id_key UNIQUE (chat_id);
CREATE INDEX idx_message_counters_chat_id ON message_counters(chat_id);

-- Remove topic_id column
ALTER TABLE message_counters DROP COLUMN IF EXISTS topic_id;