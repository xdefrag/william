-- +goose Up
-- Add unique constraint for chat summaries to support ON CONFLICT
ALTER TABLE chat_summaries 
ADD CONSTRAINT unique_chat_summary UNIQUE (chat_id);

-- +goose Down
-- Remove unique constraint
ALTER TABLE chat_summaries 
DROP CONSTRAINT IF EXISTS unique_chat_summary;
