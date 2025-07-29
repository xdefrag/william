package context

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
)

// Summarizer handles message summarization
type Summarizer struct {
	repo      *repo.Repository
	gptClient *gpt.Client
	logger    *slog.Logger
}

// NewSummarizer creates a new summarizer
func NewSummarizer(repo *repo.Repository, gptClient *gpt.Client, logger *slog.Logger) *Summarizer {
	return &Summarizer{
		repo:      repo,
		gptClient: gptClient,
		logger:    logger.WithGroup("summarizer"),
	}
}

// SummarizeChat summarizes recent messages for a chat
func (s *Summarizer) SummarizeChat(ctx context.Context, chatID int64, maxMessages int) error {
	// Get recent messages
	messages, err := s.repo.GetLatestMessagesByChatID(ctx, chatID, maxMessages)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return nil // Nothing to summarize
	}

	// Reverse messages to chronological order
	// Reverse to process from oldest to newest
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// Get existing chat summary
	existingChatSummary, err := s.repo.GetLatestChatSummary(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to get existing chat summary: %w", err)
	}

	// Get unique user IDs from messages
	userIDs := make(map[int64]bool)
	for _, msg := range messages {
		userIDs[msg.UserID] = true
	}

	// Get existing user summaries for all users in the messages
	existingUserSummaries := make(map[int64]*models.UserSummary)
	for userID := range userIDs {
		userSummary, err := s.repo.GetLatestUserSummary(ctx, chatID, userID)
		if err != nil {
			// Log error but continue - missing user summary is not critical
			s.logger.Error("Failed to get user summary for user", slog.Int64("user_id", userID), slog.Int64("chat_id", chatID), slog.String("error", err.Error()))
			continue
		}
		if userSummary != nil {
			existingUserSummaries[userID] = userSummary
		}
	}

	// Call GPT for summarization with existing data
	req := gpt.SummarizeRequest{
		ChatID:                chatID,
		Messages:              messages,
		ExistingChatSummary:   existingChatSummary,
		ExistingUserSummaries: existingUserSummaries,
	}

	response, err := s.gptClient.Summarize(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to summarize with GPT: %w", err)
	}

	// Save chat summary
	chatSummary := &models.ChatSummary{
		ChatID:     chatID,
		Summary:    response.ChatSummary.Summary,
		TopicsJSON: make(map[string]interface{}),
	}

	// Convert topics to interface{}
	for topic, count := range response.ChatSummary.Topics {
		chatSummary.TopicsJSON[topic] = count
	}

	// Add next events if present
	if response.ChatSummary.NextEvents != "" {
		chatSummary.NextEvents = &response.ChatSummary.NextEvents
	}

	err = s.repo.SaveChatSummary(ctx, chatSummary)
	if err != nil {
		return fmt.Errorf("failed to save chat summary: %w", err)
	}

	// Save user summaries
	for userIDStr, profile := range response.UserProfiles {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid user IDs
		}

		userSummary := &models.UserSummary{
			ChatID:           chatID,
			UserID:           userID,
			LikesJSON:        make(map[string]interface{}),
			DislikesJSON:     make(map[string]interface{}),
			CompetenciesJSON: make(map[string]interface{}),
		}

		// Convert likes to interface{}
		for topic, score := range profile.Likes {
			userSummary.LikesJSON[topic] = score
		}

		// Convert dislikes to interface{}
		for topic, score := range profile.Dislikes {
			userSummary.DislikesJSON[topic] = score
		}

		// Convert competencies to interface{}
		for topic, score := range profile.Competencies {
			userSummary.CompetenciesJSON[topic] = score
		}

		// Add traits if present
		if profile.Traits != "" {
			userSummary.Traits = &profile.Traits
		}

		err = s.repo.SaveUserSummary(ctx, userSummary)
		if err != nil {
			return fmt.Errorf("failed to save user summary for user %d: %w", userID, err)
		}
	}

	return nil
}

// SummarizeAllActiveChats summarizes all chats with recent activity
func (s *Summarizer) SummarizeAllActiveChats(ctx context.Context, since time.Time, maxMessages int) error {
	chatIDs, err := s.repo.GetActiveChatIDs(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to get active chats: %w", err)
	}

	for _, chatID := range chatIDs {
		if err := s.SummarizeChat(ctx, chatID, maxMessages); err != nil {
			// Log error but continue with other chats
			s.logger.Error("Failed to summarize chat", slog.Int64("chat_id", chatID), slog.String("error", err.Error()))
		}
	}

	return nil
}
