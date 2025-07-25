package context

import (
	"context"
	"fmt"

	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
)

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
func (b *Builder) BuildContextForResponse(ctx context.Context, chatID, userID int64, userName string) (*gpt.ContextRequest, error) {
	// Get chat state for conversation continuity
	chatState, err := b.repo.GetChatState(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat state: %w", err)
	}

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

	// Get recent unsummarized messages
	var lastSummaryID int64 = 0
	if chatSummary != nil {
		lastSummaryID = chatSummary.ID
	}

	recentMessages, err := b.repo.GetMessagesAfterID(ctx, chatID, lastSummaryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// Limit to configured number of recent messages for context
	limit := b.config.App.Limits.RecentMessagesLimit
	if len(recentMessages) > limit {
		recentMessages = recentMessages[len(recentMessages)-limit:]
	}

	// Extract previous response ID from chat state
	var previousResponseID *string
	if chatState != nil && chatState.PreviousResponseID != nil {
		previousResponseID = chatState.PreviousResponseID
	}

	// Check if context expired (summaries are newer than last interaction)
	contextExpired := b.isContextExpired(chatState, chatSummary, userSummary)

	return &gpt.ContextRequest{
		ChatID:             chatID,
		PreviousResponseID: previousResponseID,
		ChatSummary:        chatSummary,
		UserSummary:        userSummary,
		RecentMessages:     recentMessages,
		UserName:           userName,
		ContextExpired:     contextExpired,
	}, nil
}

// isContextExpired checks if summaries are newer than last interaction
func (b *Builder) isContextExpired(chatState *models.ChatState, chatSummary *models.ChatSummary, userSummary *models.UserSummary) bool {
	// If no previous interaction, context is not expired
	if chatState == nil {
		return false
	}

	lastInteraction := chatState.LastInteractionAt

	// Check if chat summary is newer
	if chatSummary != nil && chatSummary.CreatedAt.After(lastInteraction) {
		return true
	}

	// Check if user summary is newer
	if userSummary != nil && userSummary.CreatedAt.After(lastInteraction) {
		return true
	}

	return false
}
