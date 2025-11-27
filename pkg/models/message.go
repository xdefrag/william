package models

import "time"

// Event represents a planned event with title and optional date
type Event struct {
	Title string `json:"title"`
	Date  string `json:"date,omitempty"` // ISO 8601 format: "2012-07-04T18:10:00.000+09:00"
}

// UserTrait represents a user trait with key-value structure
type UserTrait map[string]interface{}

// Message represents a Telegram message stored in DB
type Message struct {
	ID            int64     `json:"id" db:"id"`
	TelegramMsgID int64     `json:"telegram_msg_id" db:"telegram_msg_id"`
	ChatID        int64     `json:"chat_id" db:"chat_id"`
	UserID        int64     `json:"user_id" db:"user_id"`
	TopicID       *int64    `json:"topic_id" db:"topic_id"`
	IsBot         bool      `json:"is_bot" db:"is_bot"`
	UserFirstName string    `json:"user_first_name" db:"user_first_name"`
	UserLastName  *string   `json:"user_last_name" db:"user_last_name"`
	Username      *string   `json:"username" db:"username"`
	Text          *string   `json:"text" db:"text"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ChatSummary represents aggregated chat information
type ChatSummary struct {
	ID             int64                  `json:"id" db:"id"`
	ChatID         int64                  `json:"chat_id" db:"chat_id"`
	TopicID        *int64                 `json:"topic_id" db:"topic_id"`
	Summary        string                 `json:"summary" db:"summary"`
	TopicsJSON     map[string]interface{} `json:"topics_json" db:"topics_json"`
	NextEvents     *string                `json:"next_events" db:"next_events"`           // Legacy field for backward compatibility
	NextEventsJSON []Event                `json:"next_events_json" db:"next_events_json"` // New JSON field
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
}

// UserSummary represents user behavior analysis
type UserSummary struct {
	ID               int64                  `json:"id" db:"id"`
	ChatID           int64                  `json:"chat_id" db:"chat_id"`
	UserID           int64                  `json:"user_id" db:"user_id"`
	Username         *string                `json:"username" db:"username"`
	FirstName        *string                `json:"first_name" db:"first_name"`
	LastName         *string                `json:"last_name" db:"last_name"`
	LikesJSON        map[string]interface{} `json:"likes_json" db:"likes_json"`
	DislikesJSON     map[string]interface{} `json:"dislikes_json" db:"dislikes_json"`
	CompetenciesJSON map[string]interface{} `json:"competencies_json" db:"competencies_json"`
	Traits           *string                `json:"traits" db:"traits"`           // Legacy field for backward compatibility
	TraitsJSON       UserTrait              `json:"traits_json" db:"traits_json"` // New JSON field
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
}

// UserRole represents user role assignment in a chat
type UserRole struct {
	ID             int64      `json:"id" db:"id"`
	TelegramUserID int64      `json:"telegram_user_id" db:"telegram_user_id"`
	TelegramChatID int64      `json:"telegram_chat_id" db:"telegram_chat_id"`
	Role           string     `json:"role" db:"role"`
	ExpiresAt      *time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// AllowedChat represents a chat that is allowed to use the bot
type AllowedChat struct {
	ID        int64     `json:"id" db:"id"`
	ChatID    int64     `json:"chat_id" db:"chat_id"`
	Name      *string   `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// WelcomeMessage represents a welcome message for new chat members
type WelcomeMessage struct {
	ID        int64     `json:"id" db:"id"`
	ChatID    int64     `json:"chat_id" db:"chat_id"`
	TopicID   *int64    `json:"topic_id" db:"topic_id"`
	Message   string    `json:"message" db:"message"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
