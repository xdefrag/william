package repo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdefrag/william/pkg/models"
)

// Repository provides database operations
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a new repository instance
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// JSONB handles JSON marshaling/unmarshaling for PostgreSQL JSONB
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSONB", value)
	}

	return json.Unmarshal(bytes, j)
}

// Messages operations

func (r *Repository) SaveMessage(ctx context.Context, msg *models.Message) error {
	query := `
		INSERT INTO messages (telegram_msg_id, chat_id, user_id, text, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	return r.pool.QueryRow(ctx, query, msg.TelegramMsgID, msg.ChatID, msg.UserID, msg.Text, msg.CreatedAt).Scan(&msg.ID)
}

func (r *Repository) GetLatestMessagesByChatID(ctx context.Context, chatID int64, limit int) ([]*models.Message, error) {
	query := `
		SELECT id, telegram_msg_id, chat_id, user_id, text, created_at
		FROM messages 
		WHERE chat_id = $1 
		ORDER BY id DESC 
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, chatID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(&msg.ID, &msg.TelegramMsgID, &msg.ChatID, &msg.UserID, &msg.Text, &msg.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

func (r *Repository) GetMessagesAfterID(ctx context.Context, chatID, afterID int64) ([]*models.Message, error) {
	query := `
		SELECT id, telegram_msg_id, chat_id, user_id, text, created_at
		FROM messages 
		WHERE chat_id = $1 AND id > $2
		ORDER BY id ASC`

	rows, err := r.pool.Query(ctx, query, chatID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		err := rows.Scan(&msg.ID, &msg.TelegramMsgID, &msg.ChatID, &msg.UserID, &msg.Text, &msg.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// Chat summaries operations

func (r *Repository) SaveChatSummary(ctx context.Context, summary *models.ChatSummary) error {
	query := `
		INSERT INTO chat_summaries (chat_id, summary, topics_json, next_events, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chat_id) DO UPDATE SET
			summary = EXCLUDED.summary,
			topics_json = EXCLUDED.topics_json,
			next_events = EXCLUDED.next_events,
			created_at = EXCLUDED.created_at
		RETURNING id`

	topicsJSON, err := json.Marshal(summary.TopicsJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal topics JSON: %w", err)
	}

	return r.pool.QueryRow(ctx, query, summary.ChatID, summary.Summary, topicsJSON, summary.NextEvents, summary.CreatedAt).Scan(&summary.ID)
}

func (r *Repository) GetLatestChatSummary(ctx context.Context, chatID int64) (*models.ChatSummary, error) {
	query := `
		SELECT id, chat_id, summary, topics_json, next_events, created_at
		FROM chat_summaries 
		WHERE chat_id = $1 
		ORDER BY created_at DESC 
		LIMIT 1`

	row := r.pool.QueryRow(ctx, query, chatID)

	summary := &models.ChatSummary{}
	var topicsJSON []byte

	err := row.Scan(&summary.ID, &summary.ChatID, &summary.Summary, &topicsJSON, &summary.NextEvents, &summary.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(topicsJSON, &summary.TopicsJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal topics JSON: %w", err)
	}

	return summary, nil
}

// User summaries operations

func (r *Repository) SaveUserSummary(ctx context.Context, summary *models.UserSummary) error {
	query := `
		INSERT INTO user_summaries (chat_id, user_id, likes_json, dislikes_json, traits, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET
			likes_json = EXCLUDED.likes_json,
			dislikes_json = EXCLUDED.dislikes_json,
			traits = EXCLUDED.traits,
			created_at = EXCLUDED.created_at
		RETURNING id`

	likesJSON, err := json.Marshal(summary.LikesJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal likes JSON: %w", err)
	}

	dislikesJSON, err := json.Marshal(summary.DislikesJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal dislikes JSON: %w", err)
	}

	return r.pool.QueryRow(ctx, query, summary.ChatID, summary.UserID, likesJSON, dislikesJSON, summary.Traits, summary.CreatedAt).Scan(&summary.ID)
}

func (r *Repository) GetLatestUserSummary(ctx context.Context, chatID, userID int64) (*models.UserSummary, error) {
	query := `
		SELECT id, chat_id, user_id, likes_json, dislikes_json, traits, created_at
		FROM user_summaries 
		WHERE chat_id = $1 AND user_id = $2 
		ORDER BY created_at DESC 
		LIMIT 1`

	row := r.pool.QueryRow(ctx, query, chatID, userID)

	summary := &models.UserSummary{}
	var likesJSON, dislikesJSON []byte

	err := row.Scan(&summary.ID, &summary.ChatID, &summary.UserID, &likesJSON, &dislikesJSON, &summary.Traits, &summary.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(likesJSON, &summary.LikesJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal likes JSON: %w", err)
	}

	if err := json.Unmarshal(dislikesJSON, &summary.DislikesJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dislikes JSON: %w", err)
	}

	return summary, nil
}

// GetActiveChatIDs returns list of chat IDs that have recent messages
func (r *Repository) GetActiveChatIDs(ctx context.Context, since time.Time) ([]int64, error) {
	query := `
		SELECT DISTINCT chat_id 
		FROM messages 
		WHERE created_at >= $1`

	rows, err := r.pool.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, chatID)
	}

	return chatIDs, rows.Err()
}

// Chat states operations for Responses API

func (r *Repository) GetChatState(ctx context.Context, chatID int64) (*models.ChatState, error) {
	query := `
		SELECT chat_id, previous_response_id, last_interaction_at, created_at, updated_at
		FROM chat_states 
		WHERE chat_id = $1`

	row := r.pool.QueryRow(ctx, query, chatID)

	state := &models.ChatState{}
	err := row.Scan(&state.ChatID, &state.PreviousResponseID, &state.LastInteractionAt, &state.CreatedAt, &state.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return state, nil
}

func (r *Repository) SaveChatState(ctx context.Context, state *models.ChatState) error {
	query := `
		INSERT INTO chat_states (chat_id, previous_response_id, last_interaction_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chat_id) DO UPDATE SET
			previous_response_id = EXCLUDED.previous_response_id,
			last_interaction_at = EXCLUDED.last_interaction_at,
			updated_at = EXCLUDED.updated_at`

	now := time.Now()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now
	state.LastInteractionAt = now

	_, err := r.pool.Exec(ctx, query, state.ChatID, state.PreviousResponseID, state.LastInteractionAt, state.CreatedAt, state.UpdatedAt)
	return err
}

func (r *Repository) UpdateChatStateResponseID(ctx context.Context, chatID int64, responseID string) error {
	query := `
		INSERT INTO chat_states (chat_id, previous_response_id, last_interaction_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (chat_id) DO UPDATE SET
			previous_response_id = EXCLUDED.previous_response_id,
			last_interaction_at = EXCLUDED.last_interaction_at,
			updated_at = EXCLUDED.updated_at`

	now := time.Now()
	_, err := r.pool.Exec(ctx, query, chatID, responseID, now, now, now)
	return err
}
