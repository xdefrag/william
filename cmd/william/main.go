package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/mymmrac/telego"
	"github.com/samber/do"
	"github.com/xdefrag/william/internal/bot"
	"github.com/xdefrag/william/internal/config"
	williamcontext "github.com/xdefrag/william/internal/context"
	"github.com/xdefrag/william/internal/gpt"
	grpcserver "github.com/xdefrag/william/internal/grpc"
	"github.com/xdefrag/william/internal/migrations"
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

	// Initialize slog logger with JSON handler for structured logging
	slogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create watermill adapter for compatibility
	logger := watermill.NewSlogLogger(slogger)

	// Initialize dependency injection container
	injector := do.New()
	defer func() {
		if err := injector.Shutdown(); err != nil {
			logger.Error("Failed to shutdown DI container", err, nil)
		}
	}()

	// Setup all dependencies
	if err := setupDependencies(injector, cfg, logger); err != nil {
		log.Fatalf("Failed to setup dependencies: %v", err)
	}

	// Get required services from DI
	pool := do.MustInvoke[*pgxpool.Pool](injector)
	defer pool.Close()

	publisher := do.MustInvoke[message.Publisher](injector)
	subscriber := do.MustInvoke[message.Subscriber](injector)
	listener := do.MustInvoke[*bot.Listener](injector)
	handlers := do.MustInvoke[*bot.Handlers](injector)
	sched := do.MustInvoke[*scheduler.Scheduler](injector)
	grpcSrv := do.MustInvoke[*grpcserver.Server](injector)

	// Initialize message router for event handling
	eventRouter, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		log.Fatalf("Failed to create event router: %v", err)
	}

	// Subscribe to events
	setupEventSubscribers(eventRouter, subscriber, publisher, handlers, logger)

	// Start all services
	var wg sync.WaitGroup

	// Start event router
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := eventRouter.Run(ctx); err != nil {
			logger.Error("Event router stopped with error", err, nil)
		}
	}()

	// Start bot listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := listener.Start(ctx); err != nil {
			logger.Error("Bot listener stopped with error", err, nil)
		}
	}()

	// Start scheduler
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := sched.Start(ctx); err != nil {
			logger.Error("Scheduler stopped with error", err, nil)
		}
	}()

	// Start gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcSrv.Start(ctx); err != nil {
			logger.Error("gRPC server stopped with error", err, nil)
		}
	}()

	logger.Info("William bot started successfully", watermill.LogFields{
		"config_loaded": true,
		"db_connected":  true,
		"bot_ready":     true,
		"grpc_address":  grpcSrv.GetAddress(),
	})

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", watermill.LogFields{
			"signal": sig.String(),
		})
	case <-ctx.Done():
		logger.Info("Context cancelled", nil)
	}

	// Graceful shutdown
	logger.Info("Starting graceful shutdown", nil)

	// Cancel context to stop all services
	cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Graceful shutdown completed", nil)
	case <-time.After(30 * time.Second):
		logger.Error("Shutdown timeout exceeded", nil, nil)
	}

	// Close event router
	if err := eventRouter.Close(); err != nil {
		logger.Error("Failed to close event router", err, nil)
	}

	logger.Info("William bot stopped", nil)
}

// setupDependencies registers all dependencies in DI container
func setupDependencies(injector *do.Injector, cfg *config.Config, logger watermill.LoggerAdapter) error {
	// Register config
	do.ProvideValue(injector, cfg)

	// Register slog logger (extract from watermill adapter)
	do.Provide(injector, func(i *do.Injector) (*slog.Logger, error) {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})), nil
	})

	// Register watermill logger adapter for backward compatibility
	do.ProvideValue(injector, logger)

	// Register database pool
	do.Provide(injector, func(i *do.Injector) (*pgxpool.Pool, error) {
		config := do.MustInvoke[*config.Config](i)
		logger := do.MustInvoke[watermill.LoggerAdapter](i)

		// Parse connection config for migrations
		pgxConfig, err := pgx.ParseConfig(config.PostgresDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to parse database config: %w", err)
		}

		// Create database/sql connection for migrations
		sqlDB := stdlib.OpenDB(*pgxConfig)

		// Run migrations
		if err := migrations.Run(context.Background(), sqlDB); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		logger.Info("Database migrations completed successfully", nil)

		// Close sql connection after migrations
		if err := sqlDB.Close(); err != nil {
			logger.Error("Failed to close sql connection after migrations", err, nil)
		}

		// Create pgxpool connection for application use
		pool, err := pgxpool.New(context.Background(), config.PostgresDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		// Ping database to ensure connection
		if err := pool.Ping(context.Background()); err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to ping database: %w", err)
		}

		logger.Info("Connected to database", nil)
		return pool, nil
	})

	// Register repository
	do.Provide(injector, func(i *do.Injector) (*repo.Repository, error) {
		pool := do.MustInvoke[*pgxpool.Pool](i)
		return repo.New(pool), nil
	})

	// Register pub/sub - register both publisher and subscriber
	do.Provide(injector, func(i *do.Injector) (*gochannel.GoChannel, error) {
		logger := do.MustInvoke[watermill.LoggerAdapter](i)
		return gochannel.NewGoChannel(gochannel.Config{}, logger), nil
	})

	// Register publisher interface
	do.Provide(injector, func(i *do.Injector) (message.Publisher, error) {
		pubSub := do.MustInvoke[*gochannel.GoChannel](i)
		return pubSub, nil
	})

	// Register subscriber interface
	do.Provide(injector, func(i *do.Injector) (message.Subscriber, error) {
		pubSub := do.MustInvoke[*gochannel.GoChannel](i)
		return pubSub, nil
	})

	// Register GPT client
	do.Provide(injector, func(i *do.Injector) (*gpt.Client, error) {
		config := do.MustInvoke[*config.Config](i)
		logger := do.MustInvoke[*slog.Logger](i)
		return gpt.New(config.OpenAIAPIKey, config, logger), nil
	})

	// Register context builder
	do.Provide(injector, func(i *do.Injector) (*williamcontext.Builder, error) {
		repository := do.MustInvoke[*repo.Repository](i)
		gptClient := do.MustInvoke[*gpt.Client](i)
		config := do.MustInvoke[*config.Config](i)
		return williamcontext.New(repository, gptClient, config), nil
	})

	// Register context summarizer
	do.Provide(injector, func(i *do.Injector) (*williamcontext.Summarizer, error) {
		repository := do.MustInvoke[*repo.Repository](i)
		gptClient := do.MustInvoke[*gpt.Client](i)
		logger := do.MustInvoke[*slog.Logger](i)
		return williamcontext.NewSummarizer(repository, gptClient, logger), nil
	})

	// Register Telegram bot
	do.Provide(injector, func(i *do.Injector) (*telego.Bot, error) {
		config := do.MustInvoke[*config.Config](i)
		logger := do.MustInvoke[watermill.LoggerAdapter](i)

		tgBot, err := telego.NewBot(config.TelegramBotToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}

		// Get bot info
		me, err := tgBot.GetMe(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get bot info: %w", err)
		}

		logger.Info("Bot initialized", watermill.LogFields{
			"username": me.Username,
			"id":       me.ID,
		})

		return tgBot, nil
	})

	// Register bot listener
	do.Provide(injector, func(i *do.Injector) (*bot.Listener, error) {
		tgBot := do.MustInvoke[*telego.Bot](i)
		repository := do.MustInvoke[*repo.Repository](i)
		config := do.MustInvoke[*config.Config](i)
		publisher := do.MustInvoke[message.Publisher](i)
		logger := do.MustInvoke[*slog.Logger](i)

		return bot.New(tgBot, repository, config, publisher, logger), nil
	})

	// Register bot handlers
	do.Provide(injector, func(i *do.Injector) (*bot.Handlers, error) {
		tgBot := do.MustInvoke[*telego.Bot](i)
		repository := do.MustInvoke[*repo.Repository](i)
		builder := do.MustInvoke[*williamcontext.Builder](i)
		summarizer := do.MustInvoke[*williamcontext.Summarizer](i)
		gptClient := do.MustInvoke[*gpt.Client](i)
		config := do.MustInvoke[*config.Config](i)
		logger := do.MustInvoke[*slog.Logger](i)

		return bot.NewHandlers(tgBot, repository, builder, summarizer, gptClient, config, logger), nil
	})

	// Register scheduler
	do.Provide(injector, func(i *do.Injector) (*scheduler.Scheduler, error) {
		publisher := do.MustInvoke[message.Publisher](i)
		listener := do.MustInvoke[*bot.Listener](i)
		config := do.MustInvoke[*config.Config](i)
		logger := do.MustInvoke[*slog.Logger](i)

		return scheduler.New(publisher, listener, config, logger), nil
	})

	// Register gRPC server
	do.Provide(injector, func(i *do.Injector) (*grpcserver.Server, error) {
		config := do.MustInvoke[*config.Config](i)
		repository := do.MustInvoke[*repo.Repository](i)
		logger := do.MustInvoke[*slog.Logger](i)

		return grpcserver.New(config, repository, logger)
	})

	return nil
}

// setupEventSubscribers configures event subscribers for all bot events
func setupEventSubscribers(router *message.Router, subscriber message.Subscriber, publisher message.Publisher, handlers *bot.Handlers, logger watermill.LoggerAdapter) {
	// Subscribe to summarize events
	router.AddHandler(
		"summarize_handler",
		"summarize",
		subscriber,
		"summarize",
		publisher,
		func(msg *message.Message) ([]*message.Message, error) {
			err := handlers.HandleSummarizeEvent(msg)
			return nil, err
		},
	)

	// Subscribe to mention events
	router.AddHandler(
		"mention_handler",
		"mention",
		subscriber,
		"mention",
		publisher,
		func(msg *message.Message) ([]*message.Message, error) {
			err := handlers.HandleMentionEvent(msg)
			return nil, err
		},
	)

	// Subscribe to midnight events
	router.AddHandler(
		"midnight_handler",
		"midnight",
		subscriber,
		"midnight",
		publisher,
		func(msg *message.Message) ([]*message.Message, error) {
			err := handlers.HandleMidnightEvent(msg)
			return nil, err
		},
	)

	logger.Info("Event subscribers configured", watermill.LogFields{
		"handlers": []string{"summarize", "mention", "midnight"},
	})
}
