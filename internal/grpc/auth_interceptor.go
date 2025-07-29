package grpc

import (
	"context"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/xdefrag/william/internal/auth"
)

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

const (
	// TelegramUserIDKey is the context key for telegram user ID
	TelegramUserIDKey ContextKey = "telegram_user_id"
)

// authInterceptor handles JWT authentication for gRPC requests
func authInterceptor(jwtManager *auth.JWTManager, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip authentication for health check and reflection
		if isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		// Extract token from metadata
		token, err := extractTokenFromMetadata(ctx)
		if err != nil {
			logger.Warn("Authentication failed - no token",
				slog.String("method", info.FullMethod),
				slog.String("error", err.Error()),
			)
			return nil, status.Error(codes.Unauthenticated, "missing or invalid token")
		}

		// Validate token and extract claims
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			logger.Warn("Authentication failed - invalid token",
				slog.String("method", info.FullMethod),
				slog.String("error", err.Error()),
			)
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Add telegram user ID to context for use in handlers
		ctx = context.WithValue(ctx, TelegramUserIDKey, claims.TelegramUserID)

		logger.Info("Request authenticated",
			slog.String("method", info.FullMethod),
			slog.Int64("telegram_user_id", claims.TelegramUserID),
		)

		return handler(ctx, req)
	}
}

// extractTokenFromMetadata extracts JWT token from gRPC metadata
func extractTokenFromMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata not found")
	}

	authHeader := md["authorization"]
	if len(authHeader) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization header not found")
	}

	// Extract token from "Bearer <token>" format
	token := authHeader[0]
	if !strings.HasPrefix(token, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	return strings.TrimPrefix(token, "Bearer "), nil
}

// isPublicMethod checks if the method should skip authentication
func isPublicMethod(method string) bool {
	publicMethods := []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	}

	for _, publicMethod := range publicMethods {
		if method == publicMethod {
			return true
		}
	}

	return false
}
