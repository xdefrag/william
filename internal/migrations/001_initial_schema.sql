-- +goose Up
-- messages table
CREATE TABLE messages (
  id              BIGSERIAL PRIMARY KEY,
  telegram_msg_id BIGINT NOT NULL,
  chat_id         BIGINT NOT NULL,
  user_id         BIGINT NOT NULL,
  text            TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);

-- chat snapshots
CREATE TABLE chat_summaries (
  id              BIGSERIAL PRIMARY KEY,
  chat_id         BIGINT NOT NULL,
  summary         TEXT NOT NULL,
  topics_json     JSONB NOT NULL DEFAULT '{}',
  next_events     TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);

-- user snapshots
CREATE TABLE user_summaries (
  id              BIGSERIAL PRIMARY KEY,
  chat_id         BIGINT NOT NULL,
  user_id         BIGINT NOT NULL,
  likes_json      JSONB NOT NULL DEFAULT '{}',
  dislikes_json   JSONB NOT NULL DEFAULT '{}',
  traits          TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);

-- Indexes for performance
CREATE INDEX idx_messages_chat_id_id_desc ON messages(chat_id, id DESC);
CREATE INDEX idx_chat_summaries_chat_id_created_at_desc ON chat_summaries(chat_id, created_at DESC);
CREATE INDEX idx_user_summaries_chat_user_created_at_desc ON user_summaries(chat_id, user_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_user_summaries_chat_user_created_at_desc;
DROP INDEX IF EXISTS idx_chat_summaries_chat_id_created_at_desc;
DROP INDEX IF EXISTS idx_messages_chat_id_id_desc;

DROP TABLE IF EXISTS user_summaries;
DROP TABLE IF EXISTS chat_summaries;
DROP TABLE IF EXISTS messages; 