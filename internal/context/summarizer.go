package context

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
)

// Summarizer handles message summarization
type Summarizer struct {
	repo      *repo.Repository
	gptClient *gpt.Client
}

// NewSummarizer creates a new summarizer
func NewSummarizer(repo *repo.Repository, gptClient *gpt.Client) *Summarizer {
	return &Summarizer{
		repo:      repo,
		gptClient: gptClient,
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
	lo.Reverse(messages)

	// Get existing chat summary for cumulative updates
	existingChatSummary, err := s.repo.GetLatestChatSummary(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to get existing chat summary: %w", err)
	}

	// Get existing user summaries for all users in the messages
	userIDs := make([]int64, 0)
	userIDSet := make(map[int64]bool)
	for _, msg := range messages {
		if !userIDSet[msg.UserID] {
			userIDs = append(userIDs, msg.UserID)
			userIDSet[msg.UserID] = true
		}
	}

	existingUserSummaries := make(map[int64]*models.UserSummary)
	for _, userID := range userIDs {
		userSummary, err := s.repo.GetLatestUserSummary(ctx, chatID, userID)
		if err != nil {
			return fmt.Errorf("failed to get existing user summary for user %d: %w", userID, err)
		}
		if userSummary != nil {
			existingUserSummaries[userID] = userSummary
		}
	}

	// Call GPT for cumulative summarization
	req := gpt.SummarizeRequest{
		ChatID:                chatID,
		Messages:              messages,
		ExistingChatSummary:   existingChatSummary,
		ExistingUserSummaries: existingUserSummaries,
	}

	response, err := s.gptClient.SummarizeCumulative(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to summarize with GPT: %w", err)
	}

	now := time.Now()

	// Save updated chat summary
	chatSummary := &models.ChatSummary{
		ChatID:     chatID,
		Summary:    response.ChatSummary.Summary,
		TopicsJSON: response.ChatSummary.Topics,
		CreatedAt:  now,
	}

	// Add next events if present
	if response.ChatSummary.NextEvents != "" {
		chatSummary.NextEvents = &response.ChatSummary.NextEvents
	}

	err = s.repo.SaveChatSummary(ctx, chatSummary)
	if err != nil {
		return fmt.Errorf("failed to save updated chat summary: %w", err)
	}

	// Save updated user summaries
	for userIDStr, profile := range response.UserProfiles {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			continue // Skip invalid user IDs
		}

		userSummary := &models.UserSummary{
			ChatID:       chatID,
			UserID:       userID,
			LikesJSON:    profile.Likes,
			DislikesJSON: profile.Dislikes,
			CreatedAt:    now,
		}

		// Add traits if present
		if profile.Traits != "" {
			userSummary.Traits = &profile.Traits
		}

		err = s.repo.SaveUserSummary(ctx, userSummary)
		if err != nil {
			return fmt.Errorf("failed to save updated user summary for user %d: %w", userID, err)
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
			fmt.Printf("Failed to summarize chat %d: %v\n", chatID, err)
		}
	}

	return nil
}
