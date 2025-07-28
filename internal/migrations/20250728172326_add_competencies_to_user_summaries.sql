-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_summaries 
ADD COLUMN competencies_json JSONB NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_summaries 
DROP COLUMN competencies_json;
-- +goose StatementEnd 