package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
)

// Listener handles Telegram updates
type Listener struct {
	bot       *telego.Bot
	repo      *repo.Repository
	config    *config.Config
	publisher message.Publisher
	logger    *slog.Logger
}

// New creates a new bot listener
func New(bot *telego.Bot, repo *repo.Repository, cfg *config.Config, publisher message.Publisher, logger *slog.Logger) *Listener {
	return &Listener{
		bot:       bot,
		repo:      repo,
		config:    cfg,
		publisher: publisher,
		logger:    logger.WithGroup("bot.listener"),
	}
}

// Start starts listening to Telegram updates
func (l *Listener) Start(ctx context.Context) error {
	l.logger.InfoContext(ctx, "Bot listener started",
		slog.Bool("bot_ready", true),
	)

	updates, err := l.bot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get updates channel: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			l.logger.InfoContext(ctx, "Stopping bot listener")
			return nil
		case update := <-updates:
			if update.Message != nil {
				go l.handleMessage(ctx, update.Message)
			}
		}
	}
}

// getMessageText extracts text from a message, checking both Text and Caption fields
func (l *Listener) getMessageText(msg *telego.Message) string {
	if msg.Text != "" {
		return msg.Text
	}

	// Check caption if text is empty
	if msg.Caption != "" {
		return msg.Caption
	}

	return ""
}

// getTopicID extracts topic ID from message using MessageThreadID
func (l *Listener) getTopicID(msg *telego.Message) *int64 {
	// For now, always return MessageThreadID value (0 or topic ID)
	// The handler will decide whether to use it based on chat topic support
	topicID := int64(msg.MessageThreadID)
	return &topicID
}

// handleMessage processes incoming message
func (l *Listener) handleMessage(ctx context.Context, msg *telego.Message) {
	// Get text from either Text or Caption field
	messageText := l.getMessageText(msg)

	// Skip messages without text or from bots
	if messageText == "" || msg.From.IsBot {
		return
	}

	// Check if chat is allowed
	isAllowed, err := l.repo.IsAllowedChat(ctx, msg.Chat.ID)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to check allowed chat", slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
		)
		return
	}

	if !isAllowed {
		l.logger.DebugContext(ctx, "Message from non-allowed chat ignored",
			slog.Int64("chat_id", msg.Chat.ID),
			slog.String("chat_type", msg.Chat.Type),
		)
		return
	}

	// Create message model
	var lastName *string
	if msg.From.LastName != "" {
		lastName = &msg.From.LastName
	}

	var username *string
	if msg.From.Username != "" {
		username = &msg.From.Username
	}

	message := &models.Message{
		TelegramMsgID: int64(msg.MessageID),
		ChatID:        msg.Chat.ID,
		UserID:        msg.From.ID,
		TopicID:       l.getTopicID(msg),
		UserFirstName: msg.From.FirstName,
		UserLastName:  lastName,
		Username:      username,
		Text:          &messageText,
		CreatedAt:     time.Now(),
	}

	// Save message to database
	if err := l.repo.SaveMessage(ctx, message); err != nil {
		l.logger.ErrorContext(ctx, "Failed to save message", slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
			slog.Int64("user_id", msg.From.ID),
		)
		return
	}

	// Check if message is a mention or reply to bot
	isMention := l.isMentionOrReply(msg)

	if isMention {
		// Handle mention/reply in separate goroutine
		go l.handleMention(ctx, msg)
	}

	// Increment message counter and check if we need to summarize
	topicID := l.getTopicID(msg)
	count, err := l.repo.IncrementMessageCounter(ctx, msg.Chat.ID, topicID)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to increment message counter", slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
			slog.Any("topic_id", topicID),
		)
		return
	}

	l.logger.InfoContext(ctx, "Message counter incremented",
		slog.Int64("chat_id", msg.Chat.ID),
		slog.Any("topic_id", topicID),
		slog.Int("count", count),
		slog.Int("limit", l.config.App.Limits.MaxMsgBuffer),
	)

	if count >= l.config.App.Limits.MaxMsgBuffer {
		// Reset counter and trigger summarization for this specific topic
		if err := l.repo.ResetMessageCounter(ctx, msg.Chat.ID, topicID); err != nil {
			l.logger.ErrorContext(ctx, "Failed to reset message counter", slog.Any("error", err),
				slog.Int64("chat_id", msg.Chat.ID),
				slog.Any("topic_id", topicID),
			)
			return
		}

		l.logger.InfoContext(ctx, "Triggering topic-specific summarization",
			slog.Int64("chat_id", msg.Chat.ID),
			slog.Any("topic_id", topicID),
		)

		// Publish summarization event for specific topic
		if err := l.publishSummarizeEvent(ctx, msg.Chat.ID, topicID); err != nil {
			l.logger.ErrorContext(ctx, "Failed to publish summarize event", slog.Any("error", err),
				slog.Int64("chat_id", msg.Chat.ID),
				slog.Any("topic_id", topicID),
			)
		} else {
			l.logger.InfoContext(ctx, "Summarize event published successfully",
				slog.Int64("chat_id", msg.Chat.ID),
				slog.Any("topic_id", topicID),
			)
		}
	}
}

// isMentionOrReply checks if message mentions the bot or is a reply to bot
func (l *Listener) isMentionOrReply(msg *telego.Message) bool {
	// Check for bot mention
	for _, entity := range msg.Entities {
		if entity.Type == "mention" {
			mentionText := msg.Text[entity.Offset : entity.Offset+entity.Length]
			if mentionText == l.config.App.App.MentionUsername {
				return true
			}
		}
	}

	// Check if it's a reply to bot message
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		// Get bot info to check username
		ctx := context.Background()
		botInfo, err := l.bot.GetMe(ctx)
		if err != nil {
			l.logger.ErrorContext(ctx, "Failed to get bot info", slog.Any("error", err),
				slog.Int64("chat_id", msg.Chat.ID),
				slog.Int64("user_id", msg.From.ID),
			)
			return false
		}
		return msg.ReplyToMessage.From.IsBot && msg.ReplyToMessage.From.Username == botInfo.Username
	}

	return false
}

// handleMention handles mentions and replies to the bot
func (l *Listener) handleMention(ctx context.Context, msg *telego.Message) {
	topicID := l.getTopicID(msg)
	l.logger.InfoContext(ctx, "Handling mention",
		slog.Int64("chat_id", msg.Chat.ID),
		slog.Int64("user_id", msg.From.ID),
		slog.Any("topic_id", topicID),
		slog.Int("message_thread_id", msg.MessageThreadID),
	)

	// Publish mention event for handler to process
	if err := l.publishMentionEvent(ctx, msg); err != nil {
		l.logger.ErrorContext(ctx, "Failed to publish mention event", slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
			slog.Int64("user_id", msg.From.ID),
			slog.Any("topic_id", topicID),
		)
	}
}

// publishSummarizeEvent publishes event to trigger summarization
func (l *Listener) publishSummarizeEvent(ctx context.Context, chatID int64, topicID *int64) error {
	event := SummarizeEvent{
		ChatID:    chatID,
		TopicID:   topicID,
		Timestamp: time.Now(),
	}

	msgData, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal summarize event: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), msgData)
	return l.publisher.Publish("summarize", msg)
}

// publishMentionEvent publishes event to handle mention
func (l *Listener) publishMentionEvent(ctx context.Context, msg *telego.Message) error {
	// Build username string
	username := ""
	if msg.From.Username != "" {
		username = msg.From.Username
	}

	// Build last name string
	lastName := ""
	if msg.From.LastName != "" {
		lastName = msg.From.LastName
	}

	event := MentionEvent{
		ChatID:    msg.Chat.ID,
		TopicID:   l.getTopicID(msg),
		UserID:    msg.From.ID,
		UserName:  msg.From.FirstName,
		Username:  username,
		LastName:  lastName,
		MessageID: int64(msg.MessageID),
		Text:      msg.Text,
		Timestamp: time.Now(),
	}

	msgData, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal mention event: %w", err)
	}

	msgWatermill := message.NewMessage(watermill.NewUUID(), msgData)
	return l.publisher.Publish("mention", msgWatermill)
}

// ResetCountersForAllChats resets message counters for all chats (used at midnight)
func (l *Listener) ResetCountersForAllChats() {
	ctx := context.Background()

	if err := l.repo.ResetAllMessageCounters(ctx); err != nil {
		l.logger.ErrorContext(ctx, "Failed to reset all message counters", slog.Any("error", err))
		return
	}

	l.logger.InfoContext(ctx, "Reset message counters for all chats")
}
