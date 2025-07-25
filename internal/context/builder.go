package context

import (
	"context"
	"fmt"

	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
)

// Builder constructs context for GPT requests
type Builder struct {
	repo      *repo.Repository
	gptClient *gpt.Client
}

// New creates a new context builder
func New(repo *repo.Repository, gptClient *gpt.Client) *Builder {
	return &Builder{
		repo:      repo,
		gptClient: gptClient,
	}
}

// BuildContextForResponse builds context for responding to user query
func (b *Builder) BuildContextForResponse(ctx context.Context, chatID, userID int64, userName string) (*gpt.ContextRequest, error) {
	// Get latest chat summary
	chatSummary, err := b.repo.GetLatestChatSummary(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat summary: %w", err)
	}

	// Get user summary
	userSummary, err := b.repo.GetLatestUserSummary(ctx, chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user summary: %w", err)
	}

	// Get recent unsummarized messages (last 20 for context)
	var lastSummaryID int64 = 0
	if chatSummary != nil {
		lastSummaryID = chatSummary.ID
	}

	recentMessages, err := b.repo.GetMessagesAfterID(ctx, chatID, lastSummaryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// Limit to last 20 messages for context
	if len(recentMessages) > 20 {
		recentMessages = recentMessages[len(recentMessages)-20:]
	}

	return &gpt.ContextRequest{
		ChatSummary:    chatSummary,
		UserSummary:    userSummary,
		RecentMessages: recentMessages,
		UserName:       userName,
	}, nil
}
