package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/samber/do"
	"github.com/xdefrag/william/internal/bot"
	"github.com/xdefrag/william/internal/config"
)

// Scheduler handles cron-based events
type Scheduler struct {
	publisher message.Publisher
	listener  *bot.Listener
	config    *config.Config
	logger    *slog.Logger

	// Channel to stop the scheduler
	stopCh chan struct{}
}

// New creates a new scheduler instance
func New(
	publisher message.Publisher,
	listener *bot.Listener,
	config *config.Config,
	logger *slog.Logger,
) *Scheduler {
	return &Scheduler{
		publisher: publisher,
		listener:  listener,
		config:    config,
		logger:    logger.WithGroup("scheduler"),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the scheduler with midnight cron job
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Starting scheduler")

	// Start midnight scheduler goroutine
	go s.runMidnightScheduler(ctx)

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		s.logger.InfoContext(ctx, "Scheduler context cancelled")
		return nil
	case <-s.stopCh:
		s.logger.InfoContext(ctx, "Scheduler stopped")
		return nil
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// runMidnightScheduler runs the midnight scheduler
func (s *Scheduler) runMidnightScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			// Check if it's midnight (00:00)
			if now.Hour() == 0 && now.Minute() == 0 {
				s.logger.InfoContext(ctx, "Midnight reached, triggering events",
					slog.Time("timestamp", now),
				)

				// Publish midnight event
				event := bot.MidnightEvent{
					TriggeredAt: now,
				}

				if err := s.publishMidnightEvent(ctx, event); err != nil {
					s.logger.ErrorContext(ctx, "Failed to publish midnight event", slog.Any("error", err))
				}

				// Reset counters after publishing event
				s.listener.ResetCountersForAllChats()
			}
		}
	}
}

// publishMidnightEvent publishes midnight event
func (s *Scheduler) publishMidnightEvent(ctx context.Context, event bot.MidnightEvent) error {
	msgData, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal midnight event: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), msgData)
	return s.publisher.Publish("midnight", msg)
}

// RegisterDI registers scheduler in DI container
func RegisterDI(container *do.Injector) {
	do.Provide(container, func(i *do.Injector) (*Scheduler, error) {
		publisher := do.MustInvoke[message.Publisher](i)
		listener := do.MustInvoke[*bot.Listener](i)
		config := do.MustInvoke[*config.Config](i)
		logger := do.MustInvoke[*slog.Logger](i)

		return New(publisher, listener, config, logger), nil
	})
}
