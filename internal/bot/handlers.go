package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
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
	logger     watermill.LoggerAdapter
}

// NewHandlers creates a new handlers instance
func NewHandlers(
	bot *telego.Bot,
	repo *repo.Repository,
	builder *williamcontext.Builder,
	summarizer *williamcontext.Summarizer,
	gptClient *gpt.Client,
	config *config.Config,
	logger watermill.LoggerAdapter,
) *Handlers {
	return &Handlers{
		bot:        bot,
		repo:       repo,
		builder:    builder,
		summarizer: summarizer,
		gptClient:  gptClient,
		config:     config,
		logger:     logger,
	}
}

// HandleSummarizeEvent handles summarization events
func (h *Handlers) HandleSummarizeEvent(msg *message.Message) error {
	event, err := UnmarshalSummarizeEvent(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal summarize event: %w", err)
	}

	h.logger.Info("Processing summarize event", watermill.LogFields{
		"chat_id": event.ChatID,
	})

	ctx := context.Background()
	if err := h.summarizer.SummarizeChat(ctx, event.ChatID, h.config.App.Limits.SummarizeMaxMessages); err != nil {
		h.logger.Error("Failed to summarize chat", err, watermill.LogFields{
			"chat_id": event.ChatID,
		})
		return err
	}

	h.logger.Info("Chat summarized successfully", watermill.LogFields{
		"chat_id": event.ChatID,
	})

	return nil
}

// HandleMentionEvent handles mention events
func (h *Handlers) HandleMentionEvent(msg *message.Message) error {
	event, err := UnmarshalMentionEvent(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal mention event: %w", err)
	}

	h.logger.Info("Processing mention event", watermill.LogFields{
		"chat_id": event.ChatID,
		"user_id": event.UserID,
	})

	ctx := context.Background()

	// Build context for response
	contextReq, err := h.builder.BuildContextForResponse(ctx, event.ChatID, event.UserID, event.UserName)
	if err != nil {
		h.logger.Error("Failed to build context", err, watermill.LogFields{
			"chat_id": event.ChatID,
			"user_id": event.UserID,
		})
		return err
	}

	// Extract user query (remove @william mention)
	userQuery := h.extractUserQuery(event.Text)
	contextReq.UserQuery = userQuery

	// Generate response
	response, err := h.gptClient.GenerateResponse(ctx, *contextReq)
	if err != nil {
		h.logger.Error("Failed to generate response", err, watermill.LogFields{
			"chat_id": event.ChatID,
			"user_id": event.UserID,
		})
		return err
	}

	// Send response
	if err := h.sendResponse(ctx, event.ChatID, event.MessageID, response); err != nil {
		h.logger.Error("Failed to send response", err, watermill.LogFields{
			"chat_id": event.ChatID,
			"user_id": event.UserID,
		})
		return err
	}

	h.logger.Info("Response sent successfully", watermill.LogFields{
		"chat_id": event.ChatID,
		"user_id": event.UserID,
	})

	return nil
}

// HandleMidnightEvent handles midnight reset events
func (h *Handlers) HandleMidnightEvent(msg *message.Message) error {
	event, err := UnmarshalMidnightEvent(msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal midnight event: %w", err)
	}

	h.logger.Info("Processing midnight event", watermill.LogFields{
		"timestamp": event.Timestamp,
	})

	ctx := context.Background()

	// Summarize all active chats
	if err := h.summarizer.SummarizeAllActiveChats(ctx, event.Timestamp.AddDate(0, 0, -1), h.config.App.Limits.SummarizeMaxMessages); err != nil {
		h.logger.Error("Failed to summarize active chats", err, nil)
		return err
	}

	h.logger.Info("Midnight summarization completed", nil)
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
