# gRPC Authentication Guide

## Overview

William bot uses JWT (JSON Web Token) authentication for its gRPC admin API. All admin operations require a valid JWT token containing the Telegram user ID of the requesting user.

## JWT Token Structure

### Claims Format

```json
{
  "telegram_user_id": 123456789,
  "iss": "william-bot",
  "sub": "admin-access",
  "exp": 1753853427,
  "nbf": 1753767027,
  "iat": 1753767027
}
```

### Required Claims

| Field | Type | Description |
|-------|------|-------------|
| `telegram_user_id` | int64 | **Required**. Telegram user ID of the authenticated user |
| `iss` | string | **Fixed**: `"william-bot"` - Token issuer |
| `sub` | string | **Fixed**: `"admin-access"` - Token subject |
| `exp` | int64 | **Required**. Token expiration timestamp (Unix) |
| `iat` | int64 | **Required**. Token issued at timestamp (Unix) |
| `nbf` | int64 | **Required**. Token not valid before timestamp (Unix) |

## Token Generation

### Environment Requirements

```bash
# Required environment variable
JWT_SECRET=your-super-secret-jwt-key-for-production-use
```

⚠️ **Security**: Use a strong secret (minimum 32 characters) in production.

### Using CLI Client

```bash
# Generate token with default 24h validity
./bin/williamc \
  --telegram-user-id 123456789 \
  generate-token \
  --duration 24h

# Generate token with custom duration
./bin/williamc \
  --telegram-user-id 123456789 \
  generate-token \
  --duration 7d
```

### Programmatic Generation

```go
package main

import (
    "time"
    "github.com/xdefrag/william/internal/auth"
)

func generateToken() (string, error) {
    jwtManager := auth.NewJWTManager("your-jwt-secret")
    return jwtManager.GenerateToken(123456789, 24*time.Hour)
}
```

## Authentication Flow

### 1. Request Headers

All authenticated requests must include the Authorization header:

```
Authorization: Bearer <jwt-token>
```

### 2. Token Validation Process

1. **Extract Token**: Server extracts token from `authorization` metadata
2. **Validate Format**: Ensures "Bearer " prefix is present
3. **Verify Signature**: Validates HMAC SHA-256 signature using JWT_SECRET
4. **Check Claims**: Validates expiration, issuer, and subject
5. **Extract User ID**: Adds `telegram_user_id` to request context

### 3. Context Integration

After successful authentication, the Telegram user ID is available in the request context:

```go
// In gRPC service handlers
telegramUserID := ctx.Value(grpc.TelegramUserIDKey).(int64)
```

## Public Endpoints

The following endpoints **do not** require authentication:

- `/grpc.health.v1.Health/Check` - Health check
- `/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo` - Server reflection

## Usage Examples

### grpcurl

```bash
# Health check (no auth required)
grpcurl -plaintext localhost:8080 grpc.health.v1.Health/Check

# Authenticated request
grpcurl -plaintext \
  -H "authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{"chat_ids": [-1001234567890]}' \
  localhost:8080 william.admin.v1.AdminService/GetChatSummary
```

### Go Client

```go
package main

import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"
    
    "github.com/xdefrag/william/pkg/adminpb"
)

func main() {
    // Connect to server
    conn, err := grpc.NewClient("localhost:8080", 
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        panic(err)
    }
    defer conn.Close()
    
    client := adminpb.NewAdminServiceClient(conn)
    
    // Add authentication to context
    ctx := metadata.AppendToOutgoingContext(context.Background(), 
        "authorization", "Bearer "+token)
    
    // Make authenticated request
    resp, err := client.GetChatSummary(ctx, &adminpb.GetChatSummaryRequest{
        ChatIds: []int64{-1001234567890},
    })
    if err != nil {
        panic(err)
    }
}
```

### CLI Client

```bash
# Most CLI commands auto-generate tokens
./bin/williamc \
  --telegram-user-id 123456789 \
  get-chat-summary \
  --chat-ids -1001234567890

# Some commands require explicit token
./bin/williamc \
  --telegram-user-id 123456789 \
  get-user-roles \
  --chat-id -1001234567890 \
  --token "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Error Handling

### Authentication Errors

| Error | Description | gRPC Code |
|-------|-------------|-----------|
| Missing token | No Authorization header | `UNAUTHENTICATED` |
| Invalid format | Wrong header format | `UNAUTHENTICATED` |
| Invalid signature | Token signature validation failed | `UNAUTHENTICATED` |
| Expired token | Token past expiration time | `UNAUTHENTICATED` |
| Invalid claims | Missing or invalid telegram_user_id | `UNAUTHENTICATED` |

### Example Error Response

```
rpc error: code = Unauthenticated desc = missing or invalid token
```

## Security Features

### Token Signing

- **Algorithm**: HMAC SHA-256 (HS256)
- **Secret**: Configurable via `JWT_SECRET` environment variable
- **Validation**: Full signature and expiration validation

### Interceptor Chain

1. **Logging**: Request/response logging with user context
2. **Authentication**: JWT validation with public method bypassing
3. **Error Handling**: Proper gRPC status codes

### Authorization Context

- User ID available via `grpc.TelegramUserIDKey` context key
- Ready for future role-based access control implementation
- Request logging includes authenticated user ID

## Development Setup

### Environment Variables

```bash
# .env file for development
JWT_SECRET=my-super-secret-jwt-key-for-testing-12345
TG_BOT_TOKEN=your_telegram_bot_token_here
OPENAI_API_KEY=your_openai_api_key_here
PG_DSN=postgresql://william:william@localhost:5432/william?sslmode=disable
```

### Testing Authentication

```bash
# Generate test token
export JWT_SECRET="test-secret-key"
TOKEN=$(./bin/williamc --telegram-user-id 123456789 generate-token --duration 1h | tail -1)

# Test authenticated endpoint
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{}' \
  localhost:8080 william.admin.v1.AdminService/GetAllowedChats
```

## Production Considerations

### Security

- Use a strong JWT secret (minimum 32 characters)
- Rotate JWT secrets periodically
- Set appropriate token expiration times
- Monitor authentication logs for suspicious activity

### Performance

- JWT validation is fast (no database lookup required)
- Tokens are stateless and can be cached by clients
- Consider shorter expiration times for sensitive operations

### Monitoring

Authentication events are logged with the following structure:

```json
{
  "level": "INFO",
  "method": "/william.admin.v1.AdminService/GetChatSummary",
  "telegram_user_id": 123456789,
  "message": "Request authenticated"
}
```

## Troubleshooting

### Common Issues

1. **"missing or invalid token"**
   - Check Authorization header format: `Bearer <token>`
   - Verify token is not expired
   - Ensure JWT_SECRET matches generation secret

2. **"invalid token"**
   - Token signature validation failed
   - Wrong JWT_SECRET used for validation
   - Token may be corrupted

3. **Connection errors**
   - Verify gRPC server is running on correct port
   - Check network connectivity
   - Ensure TLS settings match (insecure vs secure)

### Debug Commands

```bash
# Check server health
grpcurl -plaintext localhost:8080 grpc.health.v1.Health/Check

# List available services
grpcurl -plaintext localhost:8080 list

# Generate debug token
JWT_SECRET=debug ./bin/williamc --telegram-user-id 1 generate-token --duration 1m
``` 