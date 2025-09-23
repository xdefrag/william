package context

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
)

// Summarizer handles message summarization
type Summarizer struct {
	repo      *repo.Repository
	gptClient *gpt.Client
	config    *config.Config
	logger    *slog.Logger
}

// NewSummarizer creates a new summarizer
func NewSummarizer(repo *repo.Repository, gptClient *gpt.Client, config *config.Config, logger *slog.Logger) *Summarizer {
	return &Summarizer{
		repo:      repo,
		gptClient: gptClient,
		config:    config,
		logger:    logger.WithGroup("summarizer"),
	}
}

// TopicKey represents a safe key for grouping messages by topic
type TopicKey struct {
	hasValue bool
	value    int64
}

// NewTopicKey creates a TopicKey from a nullable int64
func NewTopicKey(topicID *int64) TopicKey {
	if topicID == nil {
		return TopicKey{hasValue: false}
	}
	return TopicKey{hasValue: true, value: *topicID}
}

// SummarizeChat summarizes recent messages for a chat, grouping by topic
func (s *Summarizer) SummarizeChat(ctx context.Context, chatID int64, maxMessages int) error {
	// Get recent messages
	messages, err := s.repo.GetLatestMessagesByChatID(ctx, chatID, maxMessages)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) == 0 {
		return nil // Nothing to summarize
	}

	// Group messages by topic
	topicGroups := make(map[TopicKey][]*models.Message)
	for _, msg := range messages {
		key := NewTopicKey(msg.TopicID)
		topicGroups[key] = append(topicGroups[key], msg)
	}

	// Summarize each topic group
	for topicKey, topicMessages := range topicGroups {
		if err := s.summarizeTopicMessages(ctx, chatID, topicKey, topicMessages); err != nil {
			// Log error but continue with other topics
			s.logger.Error("Failed to summarize topic messages",
				slog.Int64("chat_id", chatID),
				slog.Bool("has_topic", topicKey.hasValue),
				slog.String("error", err.Error()))
		}
	}

	return nil
}

// summarizeTopicMessages summarizes messages for a specific topic
func (s *Summarizer) summarizeTopicMessages(ctx context.Context, chatID int64, topicKey TopicKey, messages []*models.Message) error {
	// Reverse messages to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	var topicID *int64
	if topicKey.hasValue {
		topicID = &topicKey.value
	}

	// Get existing chat summary for this topic
	existingChatSummary, err := s.repo.GetLatestChatSummaryByTopic(ctx, chatID, topicID)
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
		BotName:               s.config.App.App.Name,
	}

	response, err := s.gptClient.Summarize(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to summarize with GPT: %w", err)
	}

	// Save chat summary with topic ID
	chatSummary := &models.ChatSummary{
		ChatID:     chatID,
		TopicID:    topicID,
		Summary:    response.ChatSummary.Summary,
		TopicsJSON: make(map[string]interface{}),
	}

	// Convert topics to interface{}
	for topic, count := range response.ChatSummary.Topics {
		chatSummary.TopicsJSON[topic] = count
	}

	// Add next events if present
	if len(response.ChatSummary.NextEvents) > 0 {
		chatSummary.NextEventsJSON = response.ChatSummary.NextEvents
	}

	err = s.repo.SaveChatSummary(ctx, chatSummary)
	if err != nil {
		return fmt.Errorf("failed to save chat summary: %w", err)
	}

	// Create user info map from messages for quick lookup
	userInfoMap := make(map[int64]*models.Message)
	for _, msg := range messages {
		if _, exists := userInfoMap[msg.UserID]; !exists {
			userInfoMap[msg.UserID] = msg
		}
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

		// Set user info from message data
		if userInfo, exists := userInfoMap[userID]; exists {
			userSummary.Username = userInfo.Username
			userSummary.FirstName = &userInfo.UserFirstName
			userSummary.LastName = userInfo.UserLastName
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
		if len(profile.Traits) > 0 {
			userSummary.TraitsJSON = profile.Traits
		}

		err = s.repo.SaveUserSummary(ctx, userSummary)
		if err != nil {
			return fmt.Errorf("failed to save user summary for user %d: %w", userID, err)
		}
	}

	return nil
}

// SummarizeChatTopic summarizes messages for a specific chat topic
func (s *Summarizer) SummarizeChatTopic(ctx context.Context, chatID int64, topicID *int64, maxMessages int) error {
	// Get recent messages for this specific topic
	var messages []*models.Message

	if topicID != nil {
		// Get messages from specific topic using GetLatestMessagesByChatID and filter
		allMessages, err := s.repo.GetLatestMessagesByChatID(ctx, chatID, maxMessages)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		// Filter messages by topic
		for _, msg := range allMessages {
			if msg.TopicID != nil && topicID != nil && *msg.TopicID == *topicID {
				messages = append(messages, msg)
			}
		}
	} else {
		// Get general chat messages (topic_id IS NULL)
		allMessages, err := s.repo.GetLatestMessagesByChatID(ctx, chatID, maxMessages)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		// Filter messages without topic
		for _, msg := range allMessages {
			if msg.TopicID == nil {
				messages = append(messages, msg)
			}
		}
	}

	if len(messages) == 0 {
		return nil // Nothing to summarize
	}

	// Use existing summarizeTopicMessages method
	topicKey := NewTopicKey(topicID)
	return s.summarizeTopicMessages(ctx, chatID, topicKey, messages)
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
