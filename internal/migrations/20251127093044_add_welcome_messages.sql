-- +goose Up
CREATE TABLE welcome_messages (
  id         BIGSERIAL PRIMARY KEY,
  chat_id    BIGINT NOT NULL,
  topic_id   BIGINT,
  message    TEXT NOT NULL,
  enabled    BOOLEAN DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(chat_id, COALESCE(topic_id, 0))
);

CREATE INDEX idx_welcome_messages_chat_topic ON welcome_messages(chat_id, topic_id);

-- +goose Down
DROP INDEX IF EXISTS idx_welcome_messages_chat_topic;
DROP TABLE IF EXISTS welcome_messages;
