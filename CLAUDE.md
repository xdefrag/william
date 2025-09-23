# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

William is a Telegram community secretary bot powered by ChatGPT (gpt-4o) that automatically processes messages, creates summaries, and provides intelligent responses when mentioned. The bot uses PostgreSQL for data persistence and includes a gRPC admin API with JWT authentication.

## Development Commands

### Essential Commands
```bash
# Setup development environment (install tools)
make setup-dev

# Build applications
make build          # Build main bot
make build-cli      # Build williamc CLI client
make build-all      # Build both

# Development
make dev            # Hot reload development mode (requires air)
make run            # Run main application

# Testing & Quality
make test           # Run tests
make test-coverage  # Run tests with coverage report
make lint           # Run golangci-lint
make check          # Run lint + test + build pipeline

# Database Operations
make migrate-up     # Apply migrations
make migrate-create name=feature_name  # Create new migration
make migrate-status # Check migration status
```

### Docker Development (Recommended)
```bash
# Quick start (creates .env if missing)
make docker-compose-dev

# Standard operations
make docker-compose-up      # Start services in background
make docker-compose-migrate # Run database migrations
make docker-compose-logs    # View logs
make docker-compose-down    # Stop services
make docker-compose-clean   # Complete cleanup
```

## Architecture

### Core Components
- **Bot Layer** (`internal/bot/`): Telegram event handling via Telego library
- **Context Layer** (`internal/context/`): Message summarization and context building using GPT
- **gRPC API** (`internal/grpc/`): Admin API with JWT authentication
- **Repository** (`internal/repo/`): PostgreSQL operations via pgx/v5
- **Scheduler** (`internal/scheduler/`): Cron jobs for midnight processing
- **Configuration** (`internal/config/`): TOML + environment variable configuration

### Data Flow
1. **Message Ingestion**: Telegram → `listener.go` → Database + Message Counter
2. **Auto-Summarization**: N messages trigger → `summarizer.go` → GPT → Summary stored
3. **Mention Responses**: @mention → `builder.go` → Context + GPT → Response
4. **Midnight Processing**: Cron → Summarize all active chats → Reset counters

### Key Technologies
- **Language**: Go 1.24+
- **Database**: PostgreSQL 15 with pgx/v5 driver
- **AI**: OpenAI GPT-4o via openai-go v1.8.2
- **Telegram**: Telego v1.2.0 library
- **Events**: Watermill pub/sub for internal messaging
- **DI Container**: samber/do for dependency injection
- **Configuration**: go-toml/v2 + environment variables
- **Migrations**: goose v3 for database schema management

## Configuration

### Environment Variables (Required)
```bash
TG_BOT_TOKEN=your_telegram_bot_token
OPENAI_API_KEY=your_openai_api_key
PG_DSN=postgres://user:pass@host:port/db
JWT_SECRET=your-jwt-secret-for-admin-api
```

### TOML Configuration
App settings in `config/app.toml` include:
- Bot name and mention username
- OpenAI model settings (temperature, max tokens)
- Message buffer limits and context settings
- System prompts for summarization and responses
- Scheduler timezone and intervals
- gRPC server ports

## Database

### Migration Management
- Migrations located in `internal/migrations/`
- Use `make migrate-create name=feature` to create new migrations
- Always run `make migrate-up` after pulling schema changes
- Check status with `make migrate-status`

### Key Tables
- `messages`: All ingested Telegram messages with user info
- `chat_summaries`: GPT-generated summaries with topic tracking
- `user_summaries`: User profile data (likes, competencies, traits)
- `allowed_chats`: Whitelist of chats the bot can operate in
- `user_roles`: Role-based permissions for users

## gRPC Admin API

### Authentication
- JWT tokens required for all admin operations (except health checks)
- Use `./bin/williamc generate-token --telegram-user-id <id>` to create tokens
- Include as: `Authorization: Bearer <token>`

### CLI Client
The `williamc` CLI provides admin functionality:
```bash
# Generate auth token
./bin/williamc --telegram-user-id 123456789 generate-token --duration 24h

# Get chat summary
./bin/williamc --telegram-user-id 123456789 get-chat-summary --chat-ids -1001234567890
```

## Development Guidelines

### Dependencies
- All dependencies must be listed in `allowed-mods.txt`
- Run `make check-imports` to verify compliance
- Use `make check` before committing to run full validation pipeline

### Code Quality
- Always run `make lint` before committing
- Use `make test-coverage` to ensure adequate test coverage
- Follow existing patterns for dependency injection using samber/do

### Hot Reload Development
- Use `make dev` for automatic rebuilds (requires `air` tool)
- Config file `config/app.toml` changes require restart
- Database schema changes require migration + restart

## Production Deployment

### Docker Compose
- Use provided `docker-compose.yml` for production deployment
- Configure environment variables via `.env` file
- Run migrations with `docker-compose run --rm migrate`
- Monitor logs with `docker-compose logs -f william`

### Security
- Use strong JWT secrets (minimum 32 characters)
- Rotate JWT secrets periodically
- Monitor authentication logs for suspicious activity
- Keep OpenAI API keys and bot tokens secure