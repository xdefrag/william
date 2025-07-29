package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/xdefrag/william/internal/auth"
	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/adminpb"
)

// Server represents the gRPC server
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	config     *config.Config
	repo       *repo.Repository
	logger     *slog.Logger
}

// New creates a new gRPC server instance
func New(cfg *config.Config, repository *repo.Repository, publisher message.Publisher, logger *slog.Logger) (*Server, error) {
	// Create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.App.GRPC.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", cfg.App.GRPC.Port, err)
	}

	// Create JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Create gRPC server with interceptors
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(logger),
			authInterceptor(jwtManager, logger),
			errorHandlingInterceptor(logger),
		),
	)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register admin service
	adminService := NewAdminService(repository, publisher, logger)
	adminpb.RegisterAdminServiceServer(server, adminService)

	// Enable server reflection for development
	reflection.Register(server)

	return &Server{
		grpcServer: server,
		listener:   listener,
		config:     cfg,
		repo:       repository,
		logger:     logger,
	}, nil
}

// Start starts the gRPC server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting gRPC server", slog.String("address", s.listener.Addr().String()))

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.grpcServer.Serve(s.listener); err != nil {
			errChan <- fmt.Errorf("gRPC server failed: %w", err)
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down gRPC server...")

		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			s.logger.Info("gRPC server shut down gracefully")
		case <-shutdownCtx.Done():
			s.logger.Warn("gRPC server forced shutdown due to timeout")
			s.grpcServer.Stop()
		}

		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

// GetAddress returns the server address
func (s *Server) GetAddress() string {
	return s.listener.Addr().String()
}
