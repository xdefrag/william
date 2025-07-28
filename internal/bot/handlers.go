package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/config"
	williamcontext "github.com/xdefrag/william/internal/context"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
)

// Handlers handles bot events
type Handlers struct {
	bot        *telego.Bot
	repo       *repo.Repository
	builder    *williamcontext.Builder
	summarizer *williamcontext.Summarizer
	gptClient  *gpt.Client
	config     *config.Config
	logger     *slog.Logger
}

// NewHandlers creates a new handlers instance
func NewHandlers(
	bot *telego.Bot,
	repo *repo.Repository,
	builder *williamcontext.Builder,
	summarizer *williamcontext.Summarizer,
	gptClient *gpt.Client,
	config *config.Config,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		bot:        bot,
		repo:       repo,
		builder:    builder,
		summarizer: summarizer,
		gptClient:  gptClient,
		config:     config,
		logger:     logger.WithGroup("bot.handlers"),
	}
}

// HandleSummarizeEvent handles summarization events
func (h *Handlers) HandleSummarizeEvent(msg *message.Message) error {
	ctx := context.Background()

	event, err := UnmarshalSummarizeEvent(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal summarize event: %w", err)
	}

	h.logger.InfoContext(ctx, "Processing summarize event",
		slog.Int64("chat_id", event.ChatID),
	)

	// Perform summarization
	if err := h.summarizer.SummarizeChat(ctx, event.ChatID, h.config.App.Limits.SummarizeMaxMessages); err != nil {
		h.logger.ErrorContext(ctx, "Failed to summarize chat", slog.Any("error", err),
			slog.Int64("chat_id", event.ChatID),
		)
		return fmt.Errorf("failed to summarize chat: %w", err)
	}

	h.logger.InfoContext(ctx, "Chat summarized successfully",
		slog.Int64("chat_id", event.ChatID),
	)

	return nil
}

// HandleMentionEvent handles mention events
func (h *Handlers) HandleMentionEvent(msg *message.Message) error {
	ctx := context.Background()

	event, err := UnmarshalMentionEvent(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal mention event: %w", err)
	}

	h.logger.InfoContext(ctx, "Processing mention event",
		slog.Int64("chat_id", event.ChatID),
		slog.Int64("user_id", event.UserID),
		slog.String("user_name", event.UserName),
	)

	// Build context for the mention
	contextReq, err := h.builder.BuildContextForResponse(ctx, event.ChatID, event.UserID, event.UserName)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to build context", slog.Any("error", err),
			slog.Int64("chat_id", event.ChatID),
			slog.Int64("user_id", event.UserID),
		)
		return fmt.Errorf("failed to build context: %w", err)
	}

	// Extract user query (remove @william mention)
	userQuery := h.extractUserQuery(event.Text)
	contextReq.UserQuery = userQuery

	// Generate response
	response, err := h.gptClient.GenerateResponse(ctx, *contextReq)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to generate response", slog.Any("error", err),
			slog.Int64("chat_id", event.ChatID),
			slog.Int64("user_id", event.UserID),
		)
		return fmt.Errorf("failed to generate response: %w", err)
	}

	// Send response
	if err := h.sendResponse(ctx, event.ChatID, event.MessageID, response); err != nil {
		h.logger.ErrorContext(ctx, "Failed to send response", slog.Any("error", err),
			slog.Int64("chat_id", event.ChatID),
			slog.Int64("user_id", event.UserID),
		)
		return fmt.Errorf("failed to send response: %w", err)
	}

	h.logger.InfoContext(ctx, "Response sent successfully",
		slog.Int64("chat_id", event.ChatID),
		slog.Int64("user_id", event.UserID),
		slog.String("user_name", event.UserName),
	)

	return nil
}

// HandleMidnightEvent handles midnight summarization events
func (h *Handlers) HandleMidnightEvent(msg *message.Message) error {
	ctx := context.Background()

	event, err := UnmarshalMidnightEvent(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal midnight event: %w", err)
	}

	h.logger.InfoContext(ctx, "Processing midnight event",
		slog.Time("triggered_at", event.TriggeredAt),
	)

	// Summarize all active chats and reset counters
	since := event.TriggeredAt.AddDate(0, 0, -1) // Previous day
	if err := h.summarizer.SummarizeAllActiveChats(ctx, since, h.config.App.Limits.SummarizeMaxMessages); err != nil {
		h.logger.ErrorContext(ctx, "Failed to summarize active chats", slog.Any("error", err))
		return fmt.Errorf("failed to summarize active chats: %w", err)
	}

	h.logger.InfoContext(ctx, "Midnight summarization completed")

	return nil
}

// extractUserQuery removes @william mention from the text
func (h *Handlers) extractUserQuery(text string) string {
	// Remove bot mention
	query := strings.ReplaceAll(text, h.config.App.App.MentionUsername, "")
	query = strings.TrimSpace(query)

	// If query is empty, provide default response
	if query == "" {
		query = h.config.App.App.DefaultResponse
	}

	return query
}

// sendResponse sends response message to chat
func (h *Handlers) sendResponse(ctx context.Context, chatID, replyToMessageID int64, response string) error {
	params := &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   response,
	}

	if replyToMessageID > 0 {
		params.ReplyParameters = &telego.ReplyParameters{
			MessageID: int(replyToMessageID),
		}
	}

	_, err := h.bot.SendMessage(ctx, params)
	return err
}
