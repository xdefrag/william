package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/xdefrag/william/internal/config"
)

// HTTPServer provides HTTP healthcheck endpoint
type HTTPServer struct {
	config *config.Config
	logger *slog.Logger
	server *http.Server
}

// NewHTTPServer creates new HTTP server instance
func NewHTTPServer(config *config.Config, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{
		config: config,
		logger: logger,
	}
}

// Start starts the HTTP server with healthcheck endpoint
func (s *HTTPServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Add healthcheck endpoint
	mux.HandleFunc("/healthcheck", s.healthcheckHandler)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.App.GRPC.HTTPPort),
		Handler: mux,
	}

	s.logger.Info("Starting HTTP healthcheck server", slog.Int("port", s.config.App.GRPC.HTTPPort))

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-errChan:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-ctx.Done():
		s.logger.Info("Shutting down HTTP server")

		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("HTTP server shutdown error", slog.String("error", err.Error()))
			return err
		}

		s.logger.Info("HTTP server stopped")
		return nil
	}
}

// healthcheckHandler handles /healthcheck endpoint
func (s *HTTPServer) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := `{"status":"ok","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
	w.Write([]byte(response))

	s.logger.Debug("Healthcheck endpoint accessed",
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
	)
}

// GetAddress returns the server address
func (s *HTTPServer) GetAddress() string {
	return fmt.Sprintf(":%d", s.config.App.GRPC.HTTPPort)
}
