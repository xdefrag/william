package scheduler

import (
	"context"
	"fmt"
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
	logger    watermill.LoggerAdapter

	// Channel to stop the scheduler
	stopCh chan struct{}
}

// New creates a new scheduler instance
func New(
	publisher message.Publisher,
	listener *bot.Listener,
	config *config.Config,
	logger watermill.LoggerAdapter,
) *Scheduler {
	return &Scheduler{
		publisher: publisher,
		listener:  listener,
		config:    config,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start starts the scheduler with midnight cron job
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting scheduler", nil)

	// Start midnight scheduler goroutine
	go s.runMidnightScheduler(ctx)

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		s.logger.Info("Scheduler context cancelled", nil)
		return nil
	case <-s.stopCh:
		s.logger.Info("Scheduler stopped", nil)
		return nil
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// runMidnightScheduler runs the midnight scheduler
func (s *Scheduler) runMidnightScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			// Check if it's midnight in the configured timezone
			localTime := now.In(s.config.Location)
			if s.isMidnight(localTime) {
				s.logger.Info("Midnight reached, triggering events", watermill.LogFields{
					"time": localTime.Format("2006-01-02 15:04:05"),
				})

				// Reset message counters
				s.listener.ResetCountersForAllChats()

				// Publish midnight event for summarization
				if err := s.publishMidnightEvent(ctx, localTime); err != nil {
					s.logger.Error("Failed to publish midnight event", err, nil)
				}
			}
		}
	}
}

// isMidnight checks if the given time is midnight (00:00)
func (s *Scheduler) isMidnight(t time.Time) bool {
	return t.Hour() == 0 && t.Minute() == 0
}

// publishMidnightEvent publishes midnight event
func (s *Scheduler) publishMidnightEvent(ctx context.Context, timestamp time.Time) error {
	event := bot.MidnightEvent{
		Timestamp: timestamp,
	}

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
		logger := do.MustInvoke[watermill.LoggerAdapter](i)

		return New(publisher, listener, config, logger), nil
	})
}
