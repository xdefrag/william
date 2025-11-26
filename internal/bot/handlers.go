package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/config"
	williamcontext "github.com/xdefrag/william/internal/context"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
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
		slog.Any("topic_id", event.TopicID),
	)

	// Perform topic-specific summarization
	if err := h.summarizer.SummarizeChatTopic(ctx, event.ChatID, event.TopicID, h.config.App.Limits.SummarizeMaxMessages); err != nil {
		h.logger.ErrorContext(ctx, "Failed to summarize chat topic", slog.Any("error", err),
			slog.Int64("chat_id", event.ChatID),
			slog.Any("topic_id", event.TopicID),
		)
		return fmt.Errorf("failed to summarize chat topic: %w", err)
	}

	h.logger.InfoContext(ctx, "Chat topic summarized successfully",
		slog.Int64("chat_id", event.ChatID),
		slog.Any("topic_id", event.TopicID),
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
		slog.Any("event_topic_id", event.TopicID),
	)

	// Build context for the mention
	params := williamcontext.BuildContextForResponseParams{
		ChatID:   event.ChatID,
		TopicID:  event.TopicID,
		UserID:   event.UserID,
		UserName: event.UserName,
	}

	contextReq, err := h.builder.BuildContextForResponse(ctx, params)
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

	// Add reply context if present
	contextReq.ReplyToText = event.ReplyToText
	contextReq.ReplyToIsBot = event.ReplyToIsBot
	contextReq.BotName = h.config.App.App.Name

	// Generate response
	mentionResponse, err := h.gptClient.GenerateResponse(ctx, *contextReq)
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to generate response", slog.Any("error", err),
			slog.Int64("chat_id", event.ChatID),
			slog.Int64("user_id", event.UserID),
		)
		return fmt.Errorf("failed to generate response: %w", err)
	}

	h.logger.InfoContext(ctx, "GPT response received",
		slog.Int64("chat_id", event.ChatID),
		slog.Bool("should_reply", mentionResponse.ShouldReply),
		slog.String("reaction", mentionResponse.Reaction),
	)

	// Set reaction if provided
	if mentionResponse.Reaction != "" {
		if err := h.setReaction(ctx, event.ChatID, event.MessageID, mentionResponse.Reaction); err != nil {
			h.logger.WarnContext(ctx, "Failed to set reaction", slog.Any("error", err),
				slog.Int64("chat_id", event.ChatID),
				slog.Int64("message_id", event.MessageID),
				slog.String("reaction", mentionResponse.Reaction),
			)
			// Don't return error, continue with response if needed
		}
	}

	// Send text response only if should_reply is true
	if mentionResponse.ShouldReply && mentionResponse.Response != "" {
		if err := h.sendResponse(ctx, event.ChatID, event.TopicID, event.MessageID, mentionResponse.Response); err != nil {
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
	} else {
		h.logger.InfoContext(ctx, "No text response needed",
			slog.Int64("chat_id", event.ChatID),
			slog.Int64("user_id", event.UserID),
			slog.Bool("should_reply", mentionResponse.ShouldReply),
		)
	}

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

// sendResponse sends response message to chat and saves it to database
func (h *Handlers) sendResponse(ctx context.Context, chatID int64, topicID *int64, replyToMessageID int64, response string) error {
	h.logger.InfoContext(ctx, "Sending response",
		slog.Int64("chat_id", chatID),
		slog.Any("topic_id", topicID),
		slog.Int64("reply_to", replyToMessageID),
	)

	params := &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   response,
	}

	// Set message thread ID for topic-based chats
	if topicID != nil {
		// Check if this chat has topic support
		hasTopics, err := h.repo.IsChatTopicEnabled(ctx, chatID)
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to check topic support, using topicID as-is",
				slog.Any("error", err),
				slog.Int64("chat_id", chatID),
			)
			hasTopics = true // Default to supporting topics if check fails
		}

		if hasTopics || *topicID > 0 {
			// Chat supports topics OR this is a specific topic message
			params.MessageThreadID = int(*topicID)
			h.logger.InfoContext(ctx, "Setting MessageThreadID",
				slog.Int("message_thread_id", params.MessageThreadID),
				slog.Bool("chat_has_topics", hasTopics),
				slog.String("reason", "topic_enabled_chat"),
			)
		} else {
			h.logger.InfoContext(ctx, "Chat doesn't support topics, not setting MessageThreadID",
				slog.Int64("chat_id", chatID),
				slog.Bool("chat_has_topics", hasTopics),
			)
		}
	}

	if replyToMessageID > 0 {
		params.ReplyParameters = &telego.ReplyParameters{
			MessageID: int(replyToMessageID),
		}
	}

	sentMessage, err := h.bot.SendMessage(ctx, params)

	// If topic message failed with "message thread not found", try sending to general chat
	if err != nil && topicID != nil {
		if strings.Contains(err.Error(), "message thread not found") {
			h.logger.WarnContext(ctx, "Topic not found, falling back to general chat",
				slog.Int64("chat_id", chatID),
				slog.Any("topic_id", topicID),
				slog.String("error", err.Error()),
			)

			// Retry without topic
			fallbackParams := &telego.SendMessageParams{
				ChatID: telego.ChatID{ID: chatID},
				Text:   response,
			}

			if replyToMessageID > 0 {
				fallbackParams.ReplyParameters = &telego.ReplyParameters{
					MessageID: int(replyToMessageID),
				}
			}

			sentMessage, err = h.bot.SendMessage(ctx, fallbackParams)
			if err != nil {
				return err
			}

			// Update topicID to nil for database storage since we fell back to general chat
			topicID = nil
		} else {
			return err
		}
	} else if err != nil {
		return err
	}

	// Save bot message to database after successful sending
	if err := h.saveBotMessage(ctx, sentMessage, topicID, response); err != nil {
		h.logger.ErrorContext(ctx, "Failed to save bot message to database", slog.Any("error", err),
			slog.Int64("chat_id", chatID),
			slog.Int("message_id", sentMessage.MessageID),
		)
		// Don't return error here as the message was already sent successfully
	}

	return nil
}

// saveBotMessage saves bot message to database
func (h *Handlers) saveBotMessage(ctx context.Context, sentMessage *telego.Message, topicID *int64, responseText string) error {
	// Get bot info to populate user fields
	botInfo, err := h.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	// Create bot message model
	var botUsername *string
	if botInfo.Username != "" {
		botUsername = &botInfo.Username
	}

	botMessage := &models.Message{
		TelegramMsgID: int64(sentMessage.MessageID),
		ChatID:        sentMessage.Chat.ID,
		UserID:        botInfo.ID,
		TopicID:       topicID,
		IsBot:         true,
		UserFirstName: botInfo.FirstName,
		UserLastName:  nil, // Bots typically don't have last names
		Username:      botUsername,
		Text:          &responseText,
		CreatedAt:     time.Now(),
	}

	return h.repo.SaveMessage(ctx, botMessage)
}

// setReaction sets an emoji reaction on a message
func (h *Handlers) setReaction(ctx context.Context, chatID int64, messageID int64, emoji string) error {
	return h.bot.SetMessageReaction(ctx, &telego.SetMessageReactionParams{
		ChatID:    telego.ChatID{ID: chatID},
		MessageID: int(messageID),
		Reaction: []telego.ReactionType{
			&telego.ReactionTypeEmoji{
				Type:  "emoji",
				Emoji: emoji,
			},
		},
	})
}
