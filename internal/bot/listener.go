package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/models"
)

// MessageCounters tracks message counts per chat for triggering summarization
type MessageCounters struct {
	mu       sync.RWMutex
	counters map[int64]int
}

// NewMessageCounters creates a new message counters instance
func NewMessageCounters() *MessageCounters {
	return &MessageCounters{
		counters: make(map[int64]int),
	}
}

// Increment increments counter for a chat and returns current count
func (mc *MessageCounters) Increment(chatID int64) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.counters[chatID]++
	return mc.counters[chatID]
}

// Reset resets counter for a chat
func (mc *MessageCounters) Reset(chatID int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.counters[chatID] = 0
}

// Listener handles Telegram updates
type Listener struct {
	bot       *telego.Bot
	repo      *repo.Repository
	config    *config.Config
	publisher message.Publisher
	counters  *MessageCounters
	logger    watermill.LoggerAdapter
}

// New creates a new bot listener
func New(bot *telego.Bot, repo *repo.Repository, cfg *config.Config, publisher message.Publisher, logger watermill.LoggerAdapter) *Listener {
	return &Listener{
		bot:       bot,
		repo:      repo,
		config:    cfg,
		publisher: publisher,
		counters:  NewMessageCounters(),
		logger:    logger,
	}
}

// Start starts listening to Telegram updates
func (l *Listener) Start(ctx context.Context) error {
	updates, err := l.bot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start long polling: %w", err)
	}

	l.logger.Info("Bot listener started", nil)

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("Stopping bot listener", nil)
			// Stop is automatic when context is cancelled
			return nil
		case update := <-updates:
			if update.Message != nil {
				go l.handleMessage(ctx, update.Message)
			}
		}
	}
}

// handleMessage processes incoming message
func (l *Listener) handleMessage(ctx context.Context, msg *telego.Message) {
	// Skip messages without text or from bots
	if msg.Text == "" || msg.From.IsBot {
		return
	}

	// Create message model
	message := &models.Message{
		TelegramMsgID: int64(msg.MessageID),
		ChatID:        msg.Chat.ID,
		UserID:        msg.From.ID,
		Text:          &msg.Text,
		CreatedAt:     time.Now(),
	}

	// Save message to database
	if err := l.repo.SaveMessage(ctx, message); err != nil {
		l.logger.Error("Failed to save message", err, watermill.LogFields{
			"chat_id": msg.Chat.ID,
			"user_id": msg.From.ID,
		})
		return
	}

	// Check if message is a mention or reply to bot
	isMention := l.isMentionOrReply(msg)

	if isMention {
		// Handle mention/reply in separate goroutine
		go l.handleMention(ctx, msg)
	}

	// Increment message counter and check if we need to summarize
	count := l.counters.Increment(msg.Chat.ID)
	if count >= l.config.App.Limits.MaxMsgBuffer {
		// Reset counter and trigger summarization
		l.counters.Reset(msg.Chat.ID)

		// Publish summarization event
		if err := l.publishSummarizeEvent(ctx, msg.Chat.ID); err != nil {
			l.logger.Error("Failed to publish summarize event", err, watermill.LogFields{
				"chat_id": msg.Chat.ID,
			})
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
			l.logger.Error("Failed to get bot info", err, nil)
			return false
		}
		return msg.ReplyToMessage.From.IsBot && msg.ReplyToMessage.From.Username == botInfo.Username
	}

	return false
}

// handleMention handles mentions and replies to the bot
func (l *Listener) handleMention(ctx context.Context, msg *telego.Message) {
	// Publish mention event for handler to process
	if err := l.publishMentionEvent(ctx, msg); err != nil {
		l.logger.Error("Failed to publish mention event", err, watermill.LogFields{
			"chat_id": msg.Chat.ID,
			"user_id": msg.From.ID,
		})
	}
}

// publishSummarizeEvent publishes event to trigger summarization
func (l *Listener) publishSummarizeEvent(ctx context.Context, chatID int64) error {
	event := SummarizeEvent{
		ChatID:    chatID,
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
	event := MentionEvent{
		ChatID:    msg.Chat.ID,
		UserID:    msg.From.ID,
		UserName:  msg.From.FirstName,
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
	l.counters.mu.Lock()
	defer l.counters.mu.Unlock()

	// Clear all counters
	for chatID := range l.counters.counters {
		l.counters.counters[chatID] = 0
	}

	l.logger.Info("Reset message counters for all chats", nil)
}
