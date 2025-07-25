package models

import (
	"encoding/json"
	"time"
)

// Message represents a Telegram message
type Message struct {
	ID            int64     `json:"id"`
	TelegramMsgID int64     `json:"telegram_msg_id"`
	ChatID        int64     `json:"chat_id"`
	UserID        int64     `json:"user_id"`
	Text          *string   `json:"text,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ChatSummary represents a summarized chat state
type ChatSummary struct {
	ID         int64          `json:"id"`
	ChatID     int64          `json:"chat_id"`
	Summary    string         `json:"summary"`
	TopicsJSON map[string]int `json:"topics_json"`
	NextEvents *string        `json:"next_events,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// UserSummary represents a user behavior summary
type UserSummary struct {
	ID           int64          `json:"id"`
	ChatID       int64          `json:"chat_id"`
	UserID       int64          `json:"user_id"`
	LikesJSON    map[string]int `json:"likes_json"`
	DislikesJSON map[string]int `json:"dislikes_json"`
	Traits       *string        `json:"traits,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ChatState represents conversation state for Responses API
type ChatState struct {
	ChatID             int64     `json:"chat_id"`
	PreviousResponseID *string   `json:"previous_response_id,omitempty"`
	LastInteractionAt  time.Time `json:"last_interaction_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Marshal methods for JSONB fields
func (cs *ChatSummary) MarshalTopicsJSON() ([]byte, error) {
	return json.Marshal(cs.TopicsJSON)
}

func (cs *ChatSummary) UnmarshalTopicsJSON(data []byte) error {
	return json.Unmarshal(data, &cs.TopicsJSON)
}

func (us *UserSummary) MarshalLikesJSON() ([]byte, error) {
	return json.Marshal(us.LikesJSON)
}

func (us *UserSummary) UnmarshalLikesJSON(data []byte) error {
	return json.Unmarshal(data, &us.LikesJSON)
}

func (us *UserSummary) MarshalDislikesJSON() ([]byte, error) {
	return json.Marshal(us.DislikesJSON)
}

func (us *UserSummary) UnmarshalDislikesJSON(data []byte) error {
	return json.Unmarshal(data, &us.DislikesJSON)
}
