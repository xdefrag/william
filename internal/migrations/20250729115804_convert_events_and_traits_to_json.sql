-- +goose Up
-- Add new JSONB columns for events and traits
ALTER TABLE chat_summaries 
ADD COLUMN IF NOT EXISTS next_events_json JSONB DEFAULT '[]';

ALTER TABLE user_summaries 
ADD COLUMN IF NOT EXISTS traits_json JSONB DEFAULT '{}';

-- Migrate existing text data to JSON format where possible
-- For events: convert simple text to JSON array with title and empty date
UPDATE chat_summaries 
SET next_events_json = 
  CASE 
    WHEN next_events IS NOT NULL AND next_events != '' THEN
      jsonb_build_array(
        jsonb_build_object('title', next_events, 'date', null)
      )
    ELSE '[]'::jsonb
  END
WHERE next_events_json = '[]'::jsonb;

-- For traits: convert simple text to JSON object with description field
UPDATE user_summaries 
SET traits_json = 
  CASE 
    WHEN traits IS NOT NULL AND traits != '' THEN
      jsonb_build_object('description', traits)
    ELSE '{}'::jsonb
  END
WHERE traits_json = '{}'::jsonb;

-- +goose Down
-- Remove new JSONB columns
ALTER TABLE chat_summaries 
DROP COLUMN IF EXISTS next_events_json;

ALTER TABLE user_summaries 
DROP COLUMN IF EXISTS traits_json; 