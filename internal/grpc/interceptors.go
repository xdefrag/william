package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// loggingInterceptor logs gRPC requests and responses
func loggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		logger.Info("gRPC request started",
			slog.String("method", info.FullMethod),
		)

		resp, err := handler(ctx, req)

		duration := time.Since(start)

		if err != nil {
			st := status.Convert(err)
			logger.Error("gRPC request failed",
				slog.String("method", info.FullMethod),
				slog.Duration("duration", duration),
				slog.String("code", st.Code().String()),
				slog.String("error", st.Message()),
			)
		} else {
			logger.Info("gRPC request completed",
				slog.String("method", info.FullMethod),
				slog.Duration("duration", duration),
			)
		}

		return resp, err
	}
}

// errorHandlingInterceptor handles and converts errors to gRPC status
func errorHandlingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			// Convert internal errors to gRPC status errors
			if st := status.Convert(err); st.Code() == codes.Unknown {
				// Log internal errors that weren't properly converted
				logger.Error("Internal error in gRPC handler",
					slog.String("method", info.FullMethod),
					slog.String("error", err.Error()),
				)
				return resp, status.Error(codes.Internal, "Internal server error")
			}
		}
		return resp, err
	}
}
