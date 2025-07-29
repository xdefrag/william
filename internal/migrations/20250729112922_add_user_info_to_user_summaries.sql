-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_summaries 
ADD COLUMN username TEXT,
ADD COLUMN first_name TEXT,
ADD COLUMN last_name TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_summaries 
DROP COLUMN username,
DROP COLUMN first_name,
DROP COLUMN last_name;
-- +goose StatementEnd 