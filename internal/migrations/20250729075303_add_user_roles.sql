-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_roles (
  id                BIGSERIAL PRIMARY KEY,
  telegram_user_id  BIGINT NOT NULL,
  telegram_chat_id  BIGINT NOT NULL,
  role              VARCHAR(50) NOT NULL,
  expires_at        TIMESTAMPTZ,
  created_at        TIMESTAMPTZ DEFAULT now(),
  updated_at        TIMESTAMPTZ DEFAULT now(),
  
  -- Пользователь может иметь только одну роль в чате
  UNIQUE(telegram_user_id, telegram_chat_id)
);

-- Add indexes for performance
CREATE INDEX idx_user_roles_user_chat ON user_roles(telegram_user_id, telegram_chat_id);
CREATE INDEX idx_user_roles_chat_id ON user_roles(telegram_chat_id);
CREATE INDEX idx_user_roles_expires_at ON user_roles(expires_at) WHERE expires_at IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_roles_expires_at;
DROP INDEX IF EXISTS idx_user_roles_chat_id;
DROP INDEX IF EXISTS idx_user_roles_user_chat;
DROP TABLE IF EXISTS user_roles;
-- +goose StatementEnd 