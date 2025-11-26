package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/repo"
)

// statsType represents the type of statistics to show
type statsType string

const (
	statsTypeMsgs    statsType = "msgs"
	statsTypeChars   statsType = "chars"
	statsTypeLastMsg statsType = "lastmsg"
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
	sType := statsTypeMsgs

	for _, arg := range args {
		argLower := strings.ToLower(arg)
		switch argLower {
		case "top":
			showBottom = false
		case "bottom":
			showBottom = true
		case "msgs", "messages":
			sType = statsTypeMsgs
		case "chars", "symbols":
			sType = statsTypeChars
		case "lastmsg", "last":
			sType = statsTypeLastMsg
		default:
			if n, err := strconv.Atoi(arg); err == nil && n > 0 {
				limit = n
				if limit > maxStatsLimit {
					limit = maxStatsLimit
				}
			}
		}
	}

	var response string
	var err error

	switch sType {
	case statsTypeChars:
		response, err = l.handleCharStats(ctx, msg.Chat.ID, limit, showBottom)
	case statsTypeLastMsg:
		response, err = l.handleLastMsgStats(ctx, msg.Chat.ID, limit, showBottom)
	default:
		response, err = l.handleMessageStats(ctx, msg.Chat.ID, limit, showBottom)
	}

	if err != nil {
		l.logger.ErrorContext(ctx, "Failed to get stats",
			slog.Any("error", err),
			slog.Int64("chat_id", msg.Chat.ID),
			slog.String("type", string(sType)),
		)
		l.sendCommandError(ctx, msg, "Не удалось получить статистику")
		return
	}

	l.sendCommandResponse(ctx, msg, response)
}

// handleMessageStats handles message count statistics
func (l *Listener) handleMessageStats(ctx context.Context, chatID int64, limit int, showBottom bool) (string, error) {
	stats, err := l.repo.GetUserMessageStats(ctx, chatID, limit, showBottom)
	if err != nil {
		return "", err
	}

	if len(stats) == 0 {
		return "Статистика пока недоступна — нет данных о сообщениях.", nil
	}

	return l.formatStatsResponse(stats, showBottom, limit), nil
}

// handleCharStats handles character count statistics
func (l *Listener) handleCharStats(ctx context.Context, chatID int64, limit int, showBottom bool) (string, error) {
	stats, err := l.repo.GetUserCharStats(ctx, chatID, limit, showBottom)
	if err != nil {
		return "", err
	}

	if len(stats) == 0 {
		return "Статистика пока недоступна — нет данных о сообщениях.", nil
	}

	return l.formatCharStatsResponse(stats, showBottom), nil
}

// handleLastMsgStats handles last message time statistics
func (l *Listener) handleLastMsgStats(ctx context.Context, chatID int64, limit int, showBottom bool) (string, error) {
	stats, err := l.repo.GetUserLastMessageStats(ctx, chatID, limit, showBottom)
	if err != nil {
		return "", err
	}

	if len(stats) == 0 {
		return "Статистика пока недоступна — нет данных о сообщениях.", nil
	}

	return l.formatLastMsgStatsResponse(stats, showBottom), nil
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

// formatUserDisplay formats user info for display (generic version)
func (l *Listener) formatUserDisplay(userID int64, username *string, firstName string, lastName *string) string {
	// Build full name
	fullName := firstName
	if lastName != nil && *lastName != "" {
		fullName += " " + *lastName
	}

	// Combine username and name (without @ to avoid mentions)
	if username != nil && *username != "" {
		if fullName != "" {
			return fmt.Sprintf("%s (%s)", *username, fullName)
		}
		return *username
	}

	if fullName != "" {
		return fullName
	}

	return fmt.Sprintf("User %d", userID)
}

// formatUserDisplayName formats user info for display
func (l *Listener) formatUserDisplayName(s *repo.UserMessageStats) string {
	return l.formatUserDisplay(s.UserID, s.Username, s.FirstName, s.LastName)
}

// formatCharStatsResponse formats character statistics into a readable message
func (l *Listener) formatCharStatsResponse(stats []*repo.UserCharStats, showBottom bool) string {
	var sb strings.Builder

	if showBottom {
		sb.WriteString(fmt.Sprintf("📊 Наименее активные по символам (топ-%d)\n\n", len(stats)))
	} else {
		sb.WriteString(fmt.Sprintf("📊 Самые активные по символам (топ-%d)\n\n", len(stats)))
	}

	for i, s := range stats {
		displayName := l.formatUserDisplay(s.UserID, s.Username, s.FirstName, s.LastName)
		charWord := l.pluralize64(s.CharCount, "символ", "символа", "символов")
		sb.WriteString(fmt.Sprintf("%d. %s — %s %s\n", i+1, displayName, l.formatNumber(s.CharCount), charWord))
	}

	return sb.String()
}

// formatLastMsgStatsResponse formats last message statistics into a readable message
func (l *Listener) formatLastMsgStatsResponse(stats []*repo.UserLastMessageStats, showBottom bool) string {
	var sb strings.Builder

	if showBottom {
		sb.WriteString(fmt.Sprintf("📊 Давно не писали (топ-%d)\n\n", len(stats)))
	} else {
		sb.WriteString(fmt.Sprintf("📊 Последние отписавшиеся (топ-%d)\n\n", len(stats)))
	}

	for i, s := range stats {
		displayName := l.formatUserDisplay(s.UserID, s.Username, s.FirstName, s.LastName)
		timeAgo := l.formatTimeAgo(s.LastMessageAt)
		sb.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, displayName, timeAgo))
	}

	return sb.String()
}

// formatNumber formats a number with thousands separator
func (l *Listener) formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	if n < 1000 {
		return str
	}

	var result strings.Builder
	length := len(str)
	for i, ch := range str {
		if i > 0 && (length-i)%3 == 0 {
			result.WriteRune(' ')
		}
		result.WriteRune(ch)
	}
	return result.String()
}

// formatTimeAgo formats time as relative string
func (l *Listener) formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "только что"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d %s назад", mins, l.pluralize(mins, "минуту", "минуты", "минут"))
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d %s назад", hours, l.pluralize(hours, "час", "часа", "часов"))
	case diff < 48*time.Hour:
		return fmt.Sprintf("вчера в %s", t.Format("15:04"))
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d %s назад", days, l.pluralize(days, "день", "дня", "дней"))
	default:
		return t.Format("02.01.2006 15:04")
	}
}

// pluralize64 returns the correct Russian plural form for int64
func (l *Listener) pluralize64(n int64, one, few, many string) string {
	return l.pluralize(int(n%100), one, few, many)
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
