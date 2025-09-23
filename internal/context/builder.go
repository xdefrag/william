package context

import (
	"context"
	"fmt"

	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
)

// BuildContextForResponseParams contains parameters for building context
type BuildContextForResponseParams struct {
	ChatID   int64
	TopicID  *int64
	UserID   int64
	UserName string
}

// Builder constructs context for GPT requests
type Builder struct {
	repo      *repo.Repository
	gptClient *gpt.Client
	config    *config.Config
}

// New creates a new context builder
func New(repo *repo.Repository, gptClient *gpt.Client, cfg *config.Config) *Builder {
	return &Builder{
		repo:      repo,
		gptClient: gptClient,
		config:    cfg,
	}
}

// BuildContextForResponse builds context for responding to user query
func (b *Builder) BuildContextForResponse(ctx context.Context, params BuildContextForResponseParams) (*gpt.ContextRequest, error) {
	// Get latest chat summary (topic-specific or general)
	chatSummary, err := b.repo.GetLatestChatSummaryByTopic(ctx, params.ChatID, params.TopicID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat summary: %w", err)
	}

	// Get user summary (user summaries are chat-wide, not topic-specific)
	userSummary, err := b.repo.GetLatestUserSummary(ctx, params.ChatID, params.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user summary: %w", err)
	}

	// Get recent unsummarized messages from the topic
	var lastSummaryID int64 = 0
	if chatSummary != nil {
		lastSummaryID = chatSummary.ID
	}

	recentMessages, err := b.repo.GetMessagesAfterIDInTopic(ctx, params.ChatID, params.TopicID, lastSummaryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// Limit to configured number of recent messages for context
	limit := b.config.App.Limits.RecentMessagesLimit
	if len(recentMessages) > limit {
		recentMessages = recentMessages[len(recentMessages)-limit:]
	}

	return &gpt.ContextRequest{
		ChatSummary:    chatSummary,
		UserSummary:    userSummary,
		RecentMessages: recentMessages,
		UserName:       params.UserName,
		UserID:         params.UserID,
	}, nil
}
