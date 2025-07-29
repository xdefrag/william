package gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/pkg/models"
)

// Client wraps OpenAI client
type Client struct {
	client *openai.Client
	config *config.Config
	logger *slog.Logger
}

// New creates a new GPT client
func New(apiKey string, cfg *config.Config, logger *slog.Logger) *Client {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(0), // Disable automatic retries to prevent unnecessary API costs
	)
	return &Client{
		client: &client,
		config: cfg,
		logger: logger.WithGroup("gpt"),
	}
}

// SummarizeRequest represents request for summarization
type SummarizeRequest struct {
	ChatID                int64
	Messages              []*models.Message
	ExistingChatSummary   *models.ChatSummary
	ExistingUserSummaries map[int64]*models.UserSummary // userID -> UserSummary
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
	NextEvents []models.Event `json:"next_events"`
}

// UserProfileData contains user-level profile information
type UserProfileData struct {
	Likes        map[string]int   `json:"likes"`
	Dislikes     map[string]int   `json:"dislikes"`
	Competencies map[string]int   `json:"competencies"`
	Traits       models.UserTrait `json:"traits"`
}

// ContextRequest represents request for context-aware response
type ContextRequest struct {
	ChatSummary    *models.ChatSummary
	UserSummary    *models.UserSummary
	RecentMessages []*models.Message
	UserQuery      string
	UserName       string
	UserID         int64
}

// Summarize generates summaries for chat and users
func (c *Client) Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error) {
	// Build messages content with user identification
	var messagesText string
	for _, msg := range req.Messages {
		if msg.Text != nil {
			// Build user identification string
			userInfo := fmt.Sprintf("User ID: %d, Name: %s", msg.UserID, msg.UserFirstName)

			if msg.UserLastName != nil && *msg.UserLastName != "" {
				userInfo += fmt.Sprintf(" %s", *msg.UserLastName)
			}

			if msg.Username != nil && *msg.Username != "" {
				userInfo += fmt.Sprintf(", Username: @%s", *msg.Username)
			}

			messagesText += fmt.Sprintf("%s: %s\n", userInfo, *msg.Text)
		}
	}

	systemPrompt := c.config.App.Prompts.SummarizeSystem

	// Build enhanced user prompt with existing data
	userPrompt := fmt.Sprintf("Chat ID: %d\n\n", req.ChatID)

	// Add existing chat summary if available
	if req.ExistingChatSummary != nil {
		userPrompt += "EXISTING CHAT SUMMARY:\n"
		userPrompt += fmt.Sprintf("Summary: %s\n", req.ExistingChatSummary.Summary)

		if len(req.ExistingChatSummary.TopicsJSON) > 0 {
			topicsJSON, _ := json.Marshal(req.ExistingChatSummary.TopicsJSON)
			userPrompt += fmt.Sprintf("Topics: %s\n", string(topicsJSON))
		}

		if req.ExistingChatSummary.NextEvents != nil {
			userPrompt += fmt.Sprintf("Next events (legacy): %s\n", *req.ExistingChatSummary.NextEvents)
		}

		if len(req.ExistingChatSummary.NextEventsJSON) > 0 {
			eventsJSON, _ := json.Marshal(req.ExistingChatSummary.NextEventsJSON)
			userPrompt += fmt.Sprintf("Next events: %s\n", string(eventsJSON))
		}
		userPrompt += "\n"
	}

	// Add existing user summaries if available
	if len(req.ExistingUserSummaries) > 0 {
		userPrompt += "EXISTING USER PROFILES:\n"
		for userID, summary := range req.ExistingUserSummaries {
			userPrompt += fmt.Sprintf("User ID %d:\n", userID)

			if len(summary.LikesJSON) > 0 {
				likesJSON, _ := json.Marshal(summary.LikesJSON)
				userPrompt += fmt.Sprintf("  Likes: %s\n", string(likesJSON))
			}

			if len(summary.DislikesJSON) > 0 {
				dislikesJSON, _ := json.Marshal(summary.DislikesJSON)
				userPrompt += fmt.Sprintf("  Dislikes: %s\n", string(dislikesJSON))
			}

			if len(summary.CompetenciesJSON) > 0 {
				competenciesJSON, _ := json.Marshal(summary.CompetenciesJSON)
				userPrompt += fmt.Sprintf("  Competencies: %s\n", string(competenciesJSON))
			}

			if summary.Traits != nil {
				userPrompt += fmt.Sprintf("  Traits (legacy): %s\n", *summary.Traits)
			}

			if len(summary.TraitsJSON) > 0 {
				traitsJSON, _ := json.Marshal(summary.TraitsJSON)
				userPrompt += fmt.Sprintf("  Traits: %s\n", string(traitsJSON))
			}
			userPrompt += "\n"
		}
	}

	userPrompt += fmt.Sprintf("NEW MESSAGES:\n%s\n", messagesText)
	userPrompt += "IMPORTANT: Update and enhance the existing data with new information from the messages. Do not replace existing data, but merge and improve it."

	// Debug log prompts before sending to OpenAI
	c.logger.DebugContext(ctx, "Sending prompts to OpenAI for summarization",
		slog.Int64("chat_id", req.ChatID),
		slog.String("model", c.config.App.OpenAI.Model),
		slog.Int("max_tokens", c.config.App.OpenAI.MaxTokensSummarize),
		slog.Float64("temperature", c.config.App.OpenAI.Temperature),
	)

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:       shared.ChatModel(c.config.App.OpenAI.Model),
		MaxTokens:   openai.Int(int64(c.config.App.OpenAI.MaxTokensSummarize)),
		Temperature: openai.Float(c.config.App.OpenAI.Temperature),
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
	systemPrompt := c.config.App.Prompts.ResponseSystem

	// Add chat context
	if req.ChatSummary != nil {
		systemPrompt += fmt.Sprintf("\n\nChat context:\nSummary: %s", req.ChatSummary.Summary)

		if req.ChatSummary.NextEvents != nil {
			systemPrompt += fmt.Sprintf("\nUpcoming events (legacy): %s", *req.ChatSummary.NextEvents)
		}

		if len(req.ChatSummary.NextEventsJSON) > 0 {
			eventsJSON, _ := json.Marshal(req.ChatSummary.NextEventsJSON)
			systemPrompt += fmt.Sprintf("\nUpcoming events: %s", string(eventsJSON))
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

		if len(req.UserSummary.CompetenciesJSON) > 0 {
			competenciesJSON, _ := json.Marshal(req.UserSummary.CompetenciesJSON)
			systemPrompt += fmt.Sprintf("\nCompetencies: %s", string(competenciesJSON))
		}

		if req.UserSummary.Traits != nil {
			systemPrompt += fmt.Sprintf("\nTraits (legacy): %s", *req.UserSummary.Traits)
		}

		if len(req.UserSummary.TraitsJSON) > 0 {
			traitsJSON, _ := json.Marshal(req.UserSummary.TraitsJSON)
			systemPrompt += fmt.Sprintf("\nTraits: %s", string(traitsJSON))
		}
	}

	// Add recent messages for context
	var recentContext string
	if len(req.RecentMessages) > 0 {
		recentContext = "\n\nRecent messages:\n"
		for _, msg := range req.RecentMessages {
			if msg.Text != nil {
				// Build user identification string
				userInfo := fmt.Sprintf("User ID: %d, Name: %s", msg.UserID, msg.UserFirstName)

				if msg.UserLastName != nil && *msg.UserLastName != "" {
					userInfo += fmt.Sprintf(" %s", *msg.UserLastName)
				}

				if msg.Username != nil && *msg.Username != "" {
					userInfo += fmt.Sprintf(", Username: @%s", *msg.Username)
				}

				recentContext += fmt.Sprintf("%s: %s\n", userInfo, *msg.Text)
			}
		}
	}

	userPrompt := recentContext + fmt.Sprintf("\n\nUser query from user ID %d (%s): %s", req.UserID, req.UserName, req.UserQuery)

	// Debug log prompts before sending to OpenAI
	c.logger.DebugContext(ctx, "Sending prompts to OpenAI for response generation",
		slog.String("user_name", req.UserName),
		slog.String("model", c.config.App.OpenAI.Model),
		slog.Int("max_tokens", c.config.App.OpenAI.MaxTokensResponse),
		slog.Float64("temperature", c.config.App.OpenAI.Temperature),
	)

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:       shared.ChatModel(c.config.App.OpenAI.Model),
		MaxTokens:   openai.Int(int64(c.config.App.OpenAI.MaxTokensResponse)),
		Temperature: openai.Float(c.config.App.OpenAI.Temperature),
	})
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
