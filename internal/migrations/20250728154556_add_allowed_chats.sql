-- +goose Up
-- +goose StatementBegin
CREATE TABLE allowed_chats (
  id       BIGSERIAL PRIMARY KEY,
  chat_id  BIGINT NOT NULL UNIQUE,
  name     TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- Add index for performance
CREATE INDEX idx_allowed_chats_chat_id ON allowed_chats(chat_id);

-- Insert initial allowed chats
INSERT INTO allowed_chats (chat_id, name) VALUES 
  (4263783995, 'тестовый'),
  (-2421937711, 'панархия сейчас'),
  (-2756876200, 'мискатоник');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_allowed_chats_chat_id;
DROP TABLE IF EXISTS allowed_chats;
-- +goose StatementEnd
