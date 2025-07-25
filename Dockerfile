# Build stage
FROM golang:1.24-alpine AS builder

# Install necessary packages
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o william ./cmd/william

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create app user
RUN adduser -D -s /bin/sh appuser

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/william .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Copy config directory
COPY --from=builder /app/config ./config

# Copy allowed modules list
COPY --from=builder /app/allowed-mods.txt .

# Install goose for migrations
RUN apk add --no-cache curl && \
    curl -L https://github.com/pressly/goose/releases/latest/download/goose_linux_x86_64 -o /usr/local/bin/goose && \
    chmod +x /usr/local/bin/goose && \
    apk del curl

# Change ownership to app user
RUN chown -R appuser:appuser /app

# Switch to app user
USER appuser

# Expose port (if needed for health checks)
EXPOSE 8080

# Run migrations and start application
CMD ["./william"] 