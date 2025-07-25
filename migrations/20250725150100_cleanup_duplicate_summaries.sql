-- +goose Up
-- Clean up duplicate chat summaries - keep only the latest for each chat
DELETE FROM chat_summaries 
WHERE id NOT IN (
    SELECT MAX(id) 
    FROM chat_summaries 
    GROUP BY chat_id
);

-- Clean up duplicate user summaries - keep only the latest for each chat-user pair
DELETE FROM user_summaries 
WHERE id NOT IN (
    SELECT MAX(id) 
    FROM user_summaries 
    GROUP BY chat_id, user_id
);

-- +goose Down
-- Cannot restore deleted duplicate data
SELECT 1;
