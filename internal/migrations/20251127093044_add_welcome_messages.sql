-- +goose Up
CREATE TABLE welcome_messages (
  id         BIGSERIAL PRIMARY KEY,
  chat_id    BIGINT NOT NULL,
  topic_id   BIGINT,
  message    TEXT NOT NULL,
  enabled    BOOLEAN DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX idx_welcome_messages_chat_topic_unique ON welcome_messages(chat_id, COALESCE(topic_id, 0));

-- +goose Down
DROP INDEX IF EXISTS idx_welcome_messages_chat_topic_unique;
DROP TABLE IF EXISTS welcome_messages;
