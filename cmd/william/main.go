package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mymmrac/telego"
	"github.com/xdefrag/william/internal/bot"
	"github.com/xdefrag/william/internal/config"
	williamcontext "github.com/xdefrag/william/internal/context"
	"github.com/xdefrag/william/internal/gpt"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/internal/scheduler"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger with debug enabled
	logger := watermill.NewStdLogger(true, true)

	// Initialize database
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Ping database to ensure connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Connected to database", nil)

	// Initialize repository
	repository := repo.New(pool)

	// Initialize message publisher/subscriber
	pubSub := gochannel.NewGoChannel(gochannel.Config{}, logger)

	// Initialize GPT client
	gptClient := gpt.New(cfg.OpenAIAPIKey, cfg)

	// Initialize context components
	contextBuilder := williamcontext.New(repository, gptClient, cfg)
	contextSummarizer := williamcontext.NewSummarizer(repository, gptClient)

	// Initialize Telegram bot
	tgBot, err := telego.NewBot(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Get bot info
	me, err := tgBot.GetMe(ctx)
	if err != nil {
		log.Fatalf("Failed to get bot info: %v", err)
	}
	logger.Info("Bot initialized", watermill.LogFields{
		"username": me.Username,
		"id":       me.ID,
	})

	// Initialize bot listener
	listener := bot.New(tgBot, repository, cfg, pubSub, logger)

	// Initialize bot handlers
	handlers := bot.NewHandlers(tgBot, repository, contextBuilder, contextSummarizer, gptClient, cfg, logger)

	// Initialize scheduler
	sched := scheduler.New(pubSub, listener, cfg, logger)

	// Setup event subscriptions
	if err := setupSubscriptions(ctx, pubSub, handlers, logger); err != nil {
		log.Fatalf("Failed to setup subscriptions: %v", err)
	}

	// Start components
	var wg sync.WaitGroup

	// Start bot listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := listener.Start(ctx); err != nil {
			logger.Error("Bot listener error", err, nil)
		}
	}()

	// Start scheduler
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sched.Start(ctx); err != nil {
			logger.Error("Scheduler error", err, nil)
		}
	}()

	logger.Info("William bot started successfully", watermill.LogFields{
		"bot_username": me.Username,
	})

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down", watermill.LogFields{
			"signal": sig.String(),
		})
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down", nil)
	}

	// Cancel context to stop all goroutines
	cancel()

	// Wait for all goroutines to finish
	logger.Info("Waiting for components to stop", nil)
	wg.Wait()

	logger.Info("William bot stopped", nil)
}

// setupSubscriptions sets up event subscriptions for bot handlers
func setupSubscriptions(ctx context.Context, subscriber message.Subscriber, handlers *bot.Handlers, logger watermill.LoggerAdapter) error {
	// Subscribe to summarize events
	summarizeMessages, err := subscriber.Subscribe(ctx, "summarize")
	if err != nil {
		return err
	}

	go func() {
		for msg := range summarizeMessages {
			logger.Info("Received summarize event", watermill.LogFields{
				"message_id": msg.UUID,
				"payload":    string(msg.Payload),
			})
			if err := handlers.HandleSummarizeEvent(msg); err != nil {
				logger.Error("Failed to handle summarize event", err, nil)
			}
			msg.Ack()
		}
	}()

	// Subscribe to mention events
	mentionMessages, err := subscriber.Subscribe(ctx, "mention")
	if err != nil {
		return err
	}

	go func() {
		for msg := range mentionMessages {
			if err := handlers.HandleMentionEvent(msg); err != nil {
				logger.Error("Failed to handle mention event", err, nil)
			}
			msg.Ack()
		}
	}()

	// Subscribe to midnight events
	midnightMessages, err := subscriber.Subscribe(ctx, "midnight")
	if err != nil {
		return err
	}

	go func() {
		for msg := range midnightMessages {
			if err := handlers.HandleMidnightEvent(msg); err != nil {
				logger.Error("Failed to handle midnight event", err, nil)
			}
			msg.Ack()
		}
	}()

	logger.Info("Event subscriptions set up successfully", nil)
	return nil
}
