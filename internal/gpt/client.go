package gpt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/xdefrag/william/pkg/models"
)

// Client wraps OpenAI client
type Client struct {
	client *openai.Client
	model  string
}

// New creates a new GPT client
func New(apiKey, model string) *Client {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &Client{
		client: &client,
		model:  model,
	}
}

// SummarizeRequest represents request for summarization
type SummarizeRequest struct {
	ChatID   int64
	Messages []*models.Message
}

// SummarizeResponse represents the structured response from GPT for summarization
type SummarizeResponse struct {
	ChatSummary  ChatSummaryData            `json:"chat_summary"`
	UserProfiles map[string]UserProfileData `json:"user_profiles"`
}

// ChatSummaryData contains chat-level summary information
type ChatSummaryData struct {
	Summary    string         `json:"summary"`
	Topics     map[string]int `json:"topics"`
	NextEvents string         `json:"next_events"`
}

// UserProfileData contains user-level profile information
type UserProfileData struct {
	Likes    map[string]int `json:"likes"`
	Dislikes map[string]int `json:"dislikes"`
	Traits   string         `json:"traits"`
}

// ContextRequest represents request for context-aware response
type ContextRequest struct {
	ChatSummary    *models.ChatSummary
	UserSummary    *models.UserSummary
	RecentMessages []*models.Message
	UserQuery      string
	UserName       string
}

// Summarize generates summaries for chat and users
func (c *Client) Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error) {
	// Build messages content
	var messagesText string
	for _, msg := range req.Messages {
		if msg.Text != nil {
			messagesText += fmt.Sprintf("User %d: %s\n", msg.UserID, *msg.Text)
		}
	}

	systemPrompt := `You are a community secretary assistant. Analyze the provided chat messages and create:

1. A chat summary (max 1000 tokens) highlighting:
   - Main discussion topics with frequency counts
   - Important decisions or conclusions
   - Upcoming events or deadlines

2. User profiles for each participant with:
   - Topics they seem interested in (likes) with engagement scores
   - Topics they dislike or criticize with frequency
   - Personality traits and communication style

Respond ONLY with valid JSON in this exact format:
{
  "chat_summary": {
    "summary": "Brief summary of chat discussions...",
    "topics": {"topic1": 5, "topic2": 3},
    "next_events": "Upcoming events or deadlines..."
  },
  "user_profiles": {
    "user_id": {
      "likes": {"topic1": 4, "topic2": 2},
      "dislikes": {"topic3": 1},
      "traits": "Communication style and personality traits..."
    }
  }
}`

	userPrompt := fmt.Sprintf("Chat ID: %d\n\nMessages:\n%s", req.ChatID, messagesText)

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:       shared.ChatModel(c.model),
		MaxTokens:   openai.Int(2048),
		Temperature: openai.Float(0.7),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result SummarizeResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	return &result, nil
}

// GenerateResponse creates context-aware response for user query
func (c *Client) GenerateResponse(ctx context.Context, req ContextRequest) (string, error) {
	// Build system prompt
	systemPrompt := "You are William, the community secretary. You help manage and respond to community discussions."

	// Add chat context
	if req.ChatSummary != nil {
		systemPrompt += fmt.Sprintf("\n\nChat context:\nSummary: %s", req.ChatSummary.Summary)

		if req.ChatSummary.NextEvents != nil {
			systemPrompt += fmt.Sprintf("\nUpcoming events: %s", *req.ChatSummary.NextEvents)
		}

		if len(req.ChatSummary.TopicsJSON) > 0 {
			topicsJSON, _ := json.Marshal(req.ChatSummary.TopicsJSON)
			systemPrompt += fmt.Sprintf("\nChat topics: %s", string(topicsJSON))
		}
	}

	// Add user context
	if req.UserSummary != nil {
		systemPrompt += fmt.Sprintf("\n\nUser %s profile:", req.UserName)

		if len(req.UserSummary.LikesJSON) > 0 {
			likesJSON, _ := json.Marshal(req.UserSummary.LikesJSON)
			systemPrompt += fmt.Sprintf("\nLikes: %s", string(likesJSON))
		}

		if len(req.UserSummary.DislikesJSON) > 0 {
			dislikesJSON, _ := json.Marshal(req.UserSummary.DislikesJSON)
			systemPrompt += fmt.Sprintf("\nDislikes: %s", string(dislikesJSON))
		}

		if req.UserSummary.Traits != nil {
			systemPrompt += fmt.Sprintf("\nTraits: %s", *req.UserSummary.Traits)
		}
	}

	// Add recent messages for context
	var recentContext string
	if len(req.RecentMessages) > 0 {
		recentContext = "\n\nRecent messages:\n"
		for _, msg := range req.RecentMessages {
			if msg.Text != nil {
				recentContext += fmt.Sprintf("User %d: %s\n", msg.UserID, *msg.Text)
			}
		}
	}

	userPrompt := recentContext + "\n\nUser query: " + req.UserQuery

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:       shared.ChatModel(c.model),
		MaxTokens:   openai.Int(1024),
		Temperature: openai.Float(0.7),
	})
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
