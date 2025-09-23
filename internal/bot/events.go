package bot

import (
	"encoding/json"
	"time"
)

// SummarizeEvent represents an event to trigger summarization
type SummarizeEvent struct {
	ChatID    int64     `json:"chat_id"`
	TopicID   *int64    `json:"topic_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Marshal serializes the event to JSON
func (e SummarizeEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalSummarizeEvent deserializes JSON to SummarizeEvent
func UnmarshalSummarizeEvent(data []byte) (SummarizeEvent, error) {
	var event SummarizeEvent
	err := json.Unmarshal(data, &event)
	return event, err
}

// MentionEvent represents an event when bot is mentioned
type MentionEvent struct {
	ChatID    int64     `json:"chat_id"`
	TopicID   *int64    `json:"topic_id,omitempty"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"` // First name
	Username  string    `json:"username"`  // @username (may be empty)
	LastName  string    `json:"last_name"` // Last name (may be empty)
	MessageID int64     `json:"message_id"`
	Text      string    `json:"text"`
	UserQuery string    `json:"user_query"` // Extracted user query from text
	Timestamp time.Time `json:"timestamp"`
}

// Marshal serializes the event to JSON
func (e MentionEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalMentionEvent deserializes JSON to MentionEvent
func UnmarshalMentionEvent(data []byte) (MentionEvent, error) {
	var event MentionEvent
	err := json.Unmarshal(data, &event)
	return event, err
}

// MidnightEvent represents daily midnight reset event
type MidnightEvent struct {
	TriggeredAt time.Time `json:"triggered_at"`
}

// Marshal serializes the event to JSON
func (e MidnightEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalMidnightEvent deserializes JSON to MidnightEvent
func UnmarshalMidnightEvent(data []byte) (MidnightEvent, error) {
	var event MidnightEvent
	err := json.Unmarshal(data, &event)
	return event, err
}
