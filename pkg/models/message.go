package models

import "time"

// Message represents a Telegram message stored in DB
type Message struct {
	ID            int64     `json:"id" db:"id"`
	TelegramMsgID int64     `json:"telegram_msg_id" db:"telegram_msg_id"`
	ChatID        int64     `json:"chat_id" db:"chat_id"`
	UserID        int64     `json:"user_id" db:"user_id"`
	UserFirstName string    `json:"user_first_name" db:"user_first_name"`
	UserLastName  *string   `json:"user_last_name" db:"user_last_name"`
	Username      *string   `json:"username" db:"username"`
	Text          *string   `json:"text" db:"text"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ChatSummary represents aggregated chat information
type ChatSummary struct {
	ID         int64                  `json:"id" db:"id"`
	ChatID     int64                  `json:"chat_id" db:"chat_id"`
	Summary    string                 `json:"summary" db:"summary"`
	TopicsJSON map[string]interface{} `json:"topics_json" db:"topics_json"`
	NextEvents *string                `json:"next_events" db:"next_events"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at" db:"updated_at"`
}

// UserSummary represents user behavior analysis
type UserSummary struct {
	ID               int64                  `json:"id" db:"id"`
	ChatID           int64                  `json:"chat_id" db:"chat_id"`
	UserID           int64                  `json:"user_id" db:"user_id"`
	LikesJSON        map[string]interface{} `json:"likes_json" db:"likes_json"`
	DislikesJSON     map[string]interface{} `json:"dislikes_json" db:"dislikes_json"`
	CompetenciesJSON map[string]interface{} `json:"competencies_json" db:"competencies_json"`
	Traits           *string                `json:"traits" db:"traits"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
}
