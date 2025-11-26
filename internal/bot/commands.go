package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/repo"
)

const (
	defaultStatsLimit = 10
	maxStatsLimit     = 50
)

// handleCommand checks if message is a command and handles it
// Returns true if the message was a command (handled or not)
func (l *Listener) handleCommand(ctx context.Context, msg *telego.Message) bool {
	text := l.getMessageText(msg)
	if text == "" || !strings.HasPrefix(text, "/") {
		return false
	}

	parts := strings.Fields(text)
	if len(parts) == 0 {
		return false
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "/stats":
		go l.handleStatsCommand(ctx, msg, args)
		return true
	}

	return false
}

// handleStatsCommand handles the /stats command
func (l *Listener) handleStatsCommand(ctx context.Context, msg *telego.Message, args []string) {
	l.logger.InfoContext(ctx, "Handling stats command",
		slog.Int64("chat_id", msg.Chat.ID),
		slog.Int64("user_id", msg.From.ID),
		slog.Any("args", args),
	)

	// Parse arguments
	showBottom := false
	limit := defaultStatsLimit

	for _, arg := range args {
		argLower := strings.ToLower(arg)
		switch argLower {
		case "top":
			showBottom = false
		case "bottom":
			showBottom = true
		default:
			if n, err := strconv.Atoi(arg); err == nil && n > 0 {
				limit = n
				if limit > maxStatsLimit {
					limit = maxStatsLimit
				}
			}
		}
	}

	// Get stats from repository
	stats, err := l.repo.GetUserMessageStats(ctx, msg.Chat.ID, limit, showBottom)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to get user message stats",
			slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
		)
		l.sendCommandError(ctx, msg, "Не удалось получить статистику")
		return
	}

	if len(stats) == 0 {
		l.sendCommandResponse(ctx, msg, "Статистика пока недоступна — нет данных о сообщениях.")
		return
	}

	// Format response
	response := l.formatStatsResponse(stats, showBottom, limit)

	// Send response
	l.sendCommandResponse(ctx, msg, response)
}

// formatStatsResponse formats the stats into a readable message
func (l *Listener) formatStatsResponse(stats []*repo.UserMessageStats, showBottom bool, limit int) string {
	var sb strings.Builder

	if showBottom {
		sb.WriteString(fmt.Sprintf("📊 Наименее активные участники (топ-%d)\n\n", len(stats)))
	} else {
		sb.WriteString(fmt.Sprintf("📊 Самые активные участники (топ-%d)\n\n", len(stats)))
	}

	for i, s := range stats {
		// Format user display name
		displayName := l.formatUserDisplayName(s)

		// Format message count with proper plural form
		msgWord := l.pluralize(s.MessageCount, "сообщение", "сообщения", "сообщений")

		sb.WriteString(fmt.Sprintf("%d. %s — %d %s\n", i+1, displayName, s.MessageCount, msgWord))
	}

	return sb.String()
}

// formatUserDisplayName formats user info for display
func (l *Listener) formatUserDisplayName(s *repo.UserMessageStats) string {
	var parts []string

	// Add username if available
	if s.Username != nil && *s.Username != "" {
		parts = append(parts, "@"+*s.Username)
	}

	// Build full name
	fullName := s.FirstName
	if s.LastName != nil && *s.LastName != "" {
		fullName += " " + *s.LastName
	}

	// Combine username and name
	if len(parts) > 0 {
		if fullName != "" {
			return fmt.Sprintf("%s (%s)", parts[0], fullName)
		}
		return parts[0]
	}

	if fullName != "" {
		return fullName
	}

	return fmt.Sprintf("User %d", s.UserID)
}

// pluralize returns the correct Russian plural form
func (l *Listener) pluralize(n int, one, few, many string) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}

	// Special case for 11-19
	if abs%100 >= 11 && abs%100 <= 19 {
		return many
	}

	switch abs % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

// sendCommandResponse sends a response to a command
func (l *Listener) sendCommandResponse(ctx context.Context, msg *telego.Message, text string) {
	params := &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: msg.Chat.ID},
		Text:   text,
	}

	// Set message thread ID for topic-based chats
	if msg.MessageThreadID > 0 {
		params.MessageThreadID = msg.MessageThreadID
	}

	_, err := l.bot.SendMessage(ctx, params)
	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to send command response",
			slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
		)
	}
}

// sendCommandError sends an error response to a command
func (l *Listener) sendCommandError(ctx context.Context, msg *telego.Message, text string) {
	l.sendCommandResponse(ctx, msg, "❌ "+text)
}
