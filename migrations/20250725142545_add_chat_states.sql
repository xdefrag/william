-- +goose Up
-- chat_states table for storing conversation state with Responses API
CREATE TABLE chat_states (
  chat_id                BIGINT PRIMARY KEY,
  previous_response_id   TEXT,
  last_interaction_at    TIMESTAMPTZ DEFAULT now(),
  created_at             TIMESTAMPTZ DEFAULT now(),
  updated_at             TIMESTAMPTZ DEFAULT now()
);

-- Index for faster lookups by last interaction
CREATE INDEX idx_chat_states_last_interaction ON chat_states(last_interaction_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_chat_states_last_interaction;
DROP TABLE IF EXISTS chat_states;
