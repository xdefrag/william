package gpt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/pkg/models"
)

// Client wraps OpenAI client with Responses API support
type Client struct {
	client *openai.Client
	config *config.Config
}

// New creates a new GPT client
func New(apiKey string, cfg *config.Config) *Client {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &Client{
		client: &client,
		config: cfg,
	}
}

// SummarizeRequest represents request for summarization
type SummarizeRequest struct {
	ChatID                int64
	Messages              []*models.Message
	ExistingChatSummary   *models.ChatSummary
	ExistingUserSummaries map[int64]*models.UserSummary
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

// ContextRequest represents request for context-aware response using Responses API
type ContextRequest struct {
	ChatID             int64
	PreviousResponseID *string
	ChatSummary        *models.ChatSummary
	UserSummary        *models.UserSummary
	RecentMessages     []*models.Message
	UserQuery          string
	UserName           string
	ContextExpired     bool // True if summaries are newer than last interaction
}

// ResponseResult contains the response and updated conversation state
type ResponseResult struct {
	Text          string
	NewResponseID string
}

// Summarize generates summaries for chat and users (backward compatibility)
func (c *Client) Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error) {
	// Build messages content
	var messagesText string
	for _, msg := range req.Messages {
		if msg.Text != nil {
			messagesText += fmt.Sprintf("User %d: %s\n", msg.UserID, *msg.Text)
		}
	}

	systemPrompt := c.config.App.Prompts.SummarizeSystem
	userPrompt := fmt.Sprintf("Chat ID: %d\n\nMessages:\n%s", req.ChatID, messagesText)

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:       openai.ChatModelGPT4o,
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

// SummarizeCumulative generates updated summaries based on existing state and new messages
func (c *Client) SummarizeCumulative(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error) {
	// Build messages content
	var messagesText string
	for _, msg := range req.Messages {
		if msg.Text != nil {
			messagesText += fmt.Sprintf("User %d: %s\n", msg.UserID, *msg.Text)
		}
	}

	// Build system prompt for cumulative updates
	systemPrompt := c.config.App.Prompts.SummarizeSystem
	systemPrompt += "\n\n**IMPORTANT**: This is a CUMULATIVE UPDATE, not a fresh start. Update and enhance existing information rather than replacing it entirely."

	// Add existing chat summary context
	userPrompt := fmt.Sprintf("Chat ID: %d\n\n", req.ChatID)

	if req.ExistingChatSummary != nil {
		userPrompt += "=== EXISTING CHAT SUMMARY (to be updated) ===\n"
		userPrompt += fmt.Sprintf("Summary: %s\n", req.ExistingChatSummary.Summary)

		if len(req.ExistingChatSummary.TopicsJSON) > 0 {
			topicsJSON, _ := json.Marshal(req.ExistingChatSummary.TopicsJSON)
			userPrompt += fmt.Sprintf("Topics: %s\n", string(topicsJSON))
		}

		if req.ExistingChatSummary.NextEvents != nil {
			userPrompt += fmt.Sprintf("Next Events: %s\n", *req.ExistingChatSummary.NextEvents)
		}
		userPrompt += "\n"
	} else {
		userPrompt += "=== NO EXISTING CHAT SUMMARY ===\n\n"
	}

	// Add existing user summaries context
	if len(req.ExistingUserSummaries) > 0 {
		userPrompt += "=== EXISTING USER PROFILES (to be updated) ===\n"
		for userID, userSummary := range req.ExistingUserSummaries {
			userPrompt += fmt.Sprintf("User %d:\n", userID)

			if len(userSummary.LikesJSON) > 0 {
				likesJSON, _ := json.Marshal(userSummary.LikesJSON)
				userPrompt += fmt.Sprintf("  Likes: %s\n", string(likesJSON))
			}

			if len(userSummary.DislikesJSON) > 0 {
				dislikesJSON, _ := json.Marshal(userSummary.DislikesJSON)
				userPrompt += fmt.Sprintf("  Dislikes: %s\n", string(dislikesJSON))
			}

			if userSummary.Traits != nil {
				userPrompt += fmt.Sprintf("  Traits: %s\n", *userSummary.Traits)
			}
			userPrompt += "\n"
		}
	} else {
		userPrompt += "=== NO EXISTING USER PROFILES ===\n\n"
	}

	// Add new messages to analyze
	userPrompt += "=== NEW MESSAGES TO ANALYZE ===\n"
	userPrompt += messagesText
	userPrompt += "\n"
	userPrompt += "Please UPDATE the existing summaries and user profiles based on these new messages. "
	userPrompt += "Enhance existing information, add new insights, update topic counts, and refine user traits. "
	userPrompt += "Do not completely replace existing data - build upon it."

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Model:       openai.ChatModelGPT4o,
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

// GenerateResponse creates context-aware response using Responses API
func (c *Client) GenerateResponse(ctx context.Context, req ContextRequest) (*ResponseResult, error) {
	// Check if we should treat this as a fresh conversation
	shouldUseFreshContext := req.PreviousResponseID == nil || req.ContextExpired

	var resp *responses.Response
	var err error

	if shouldUseFreshContext {
		resp, err = c.generateFreshResponse(ctx, req)
	} else {
		resp, err = c.continueConversation(ctx, req)
	}

	if err != nil {
		return nil, err
	}

	// Extract text content from response
	responseText := c.extractResponseText(resp)
	if responseText == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	return &ResponseResult{
		Text:          responseText,
		NewResponseID: resp.ID,
	}, nil
}

// generateFreshResponse creates a new conversation with full context
func (c *Client) generateFreshResponse(ctx context.Context, req ContextRequest) (*responses.Response, error) {
	// Build system prompt with full context
	instructions := c.buildSystemInstructions(req)

	// Prepare input messages
	input := c.buildInputMessages(req)

	// Build request parameters
	params := responses.ResponseNewParams{
		Model: openai.ChatModelGPT4o,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: input,
		},
		Instructions: openai.String(instructions),
		Store:        openai.Bool(true), // Enable conversation storage
	}

	return c.client.Responses.New(ctx, params)
}

// continueConversation continues existing conversation thread
func (c *Client) continueConversation(ctx context.Context, req ContextRequest) (*responses.Response, error) {
	// For continuation, send only user query - OpenAI manages context
	input := []responses.ResponseInputItemUnionParam{
		responses.ResponseInputItemParamOfMessage(req.UserQuery, responses.EasyInputMessageRoleUser),
	}

	params := responses.ResponseNewParams{
		Model: openai.ChatModelGPT4o,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: input,
		},
		PreviousResponseID: openai.String(*req.PreviousResponseID),
		Store:              openai.Bool(true),
	}

	return c.client.Responses.New(ctx, params)
}

// buildSystemInstructions creates system instructions with context
func (c *Client) buildSystemInstructions(req ContextRequest) string {
	instructions := c.config.App.Prompts.ResponseSystem

	// Add chat context
	if req.ChatSummary != nil {
		instructions += fmt.Sprintf("\n\nChat context:\nSummary: %s", req.ChatSummary.Summary)

		if req.ChatSummary.NextEvents != nil {
			instructions += fmt.Sprintf("\nUpcoming events: %s", *req.ChatSummary.NextEvents)
		}

		if len(req.ChatSummary.TopicsJSON) > 0 {
			topicsJSON, _ := json.Marshal(req.ChatSummary.TopicsJSON)
			instructions += fmt.Sprintf("\nChat topics: %s", string(topicsJSON))
		}
	}

	// Add user context
	if req.UserSummary != nil {
		instructions += fmt.Sprintf("\n\nUser %s profile:", req.UserName)

		if len(req.UserSummary.LikesJSON) > 0 {
			likesJSON, _ := json.Marshal(req.UserSummary.LikesJSON)
			instructions += fmt.Sprintf("\nLikes: %s", string(likesJSON))
		}

		if len(req.UserSummary.DislikesJSON) > 0 {
			dislikesJSON, _ := json.Marshal(req.UserSummary.DislikesJSON)
			instructions += fmt.Sprintf("\nDislikes: %s", string(dislikesJSON))
		}

		if req.UserSummary.Traits != nil {
			instructions += fmt.Sprintf("\nTraits: %s", *req.UserSummary.Traits)
		}
	}

	return instructions
}

// buildInputMessages creates input messages for fresh conversation
func (c *Client) buildInputMessages(req ContextRequest) []responses.ResponseInputItemUnionParam {
	input := []responses.ResponseInputItemUnionParam{
		responses.ResponseInputItemParamOfMessage(req.UserQuery, responses.EasyInputMessageRoleUser),
	}

	// Add recent messages for immediate context if present
	if len(req.RecentMessages) > 0 {
		var recentContext string
		recentContext = "Recent messages:\n"
		for _, msg := range req.RecentMessages {
			if msg.Text != nil {
				recentContext += fmt.Sprintf("User %d: %s\n", msg.UserID, *msg.Text)
			}
		}

		// Add recent context as separate user message before main query
		input = []responses.ResponseInputItemUnionParam{
			responses.ResponseInputItemParamOfMessage(recentContext, responses.EasyInputMessageRoleUser),
			responses.ResponseInputItemParamOfMessage(req.UserQuery, responses.EasyInputMessageRoleUser),
		}
	}

	return input
}

// extractResponseText extracts text content from API response
func (c *Client) extractResponseText(resp *responses.Response) string {
	var responseText string
	for _, output := range resp.Output {
		if output.Type == "message" && len(output.Content) > 0 {
			for _, content := range output.Content {
				if content.Type == "output_text" && content.Text != "" {
					responseText += content.Text
				}
			}
		}
	}
	return responseText
}
