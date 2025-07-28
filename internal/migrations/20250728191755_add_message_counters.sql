-- +goose Up
-- +goose StatementBegin
CREATE TABLE message_counters (
  id       BIGSERIAL PRIMARY KEY,
  chat_id  BIGINT NOT NULL UNIQUE,
  count    INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Add index for performance
CREATE INDEX idx_message_counters_chat_id ON message_counters(chat_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_message_counters_chat_id;
DROP TABLE IF EXISTS message_counters;
-- +goose StatementEnd 