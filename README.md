# William - Telegram Community Secretary Bot

William is a Telegram bot powered by ChatGPT (gpt-4o) that serves as a secretary for Telegram communities. It listens to all messages, creates context summaries, and provides intelligent responses when mentioned.

## Features

- **Message Ingestion**: Automatically saves all messages to PostgreSQL database
- **Auto-Summarization**: Triggers GPT summarization after N messages
- **Context-Aware Responses**: Uses chat history and user profiles for intelligent replies
- **Midnight Processing**: Daily summarization and counter reset
- **TOML Configuration**: Centralized application settings

## Quick Start with Docker Compose

### Prerequisites

- Docker and Docker Compose
- Telegram Bot Token (get from [@BotFather](https://t.me/BotFather))
- OpenAI API Key

### Local Development Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd william
   ```

2. **Configure environment variables**
   ```bash
   # Copy example environment file
   cp docker-compose.env.example .env
   
   # Edit .env with your actual values
   nano .env
   ```

3. **Start the services**
   ```bash
   # Build and start PostgreSQL + William bot
   docker-compose up -d
   
   # Run database migrations
   docker-compose run --rm migrate
   
   # Check logs
   docker-compose logs -f william
   ```

4. **Stop the services**
   ```bash
   docker-compose down
   ```

### Environment Variables

Required variables for `.env` file:

```bash
# Required
TG_BOT_TOKEN=your_telegram_bot_token_here
OPENAI_API_KEY=your_openai_api_key_here

# Optional (with defaults)
OPENAI_MODEL=gpt-4o-mini
MAX_MSG_BUFFER=100
CTX_MAX_TOKENS=2048
TZ=Europe/Belgrade
```

### Docker Services

- **postgres**: PostgreSQL 15 database with persistent storage
- **william**: The bot application
- **migrate**: One-time migration runner

## Manual Development Setup

### Prerequisites

- Go 1.24+
- PostgreSQL 15+
- Required tools: `goose`, `golangci-lint`, `air` (optional)

### Setup Commands

```bash
# Install development tools
make setup-dev

# Set up database
export DB_URL="postgres://user:password@localhost/william?sslmode=disable"
make migrate-up

# Run application
export TG_BOT_TOKEN="your_token"
export OPENAI_API_KEY="your_key" 
export PG_DSN="your_db_connection"
make run

# Development with hot reload
make dev
```

## Configuration

### TOML Configuration (`config/app.toml`)

Application settings (non-secrets) are configured in TOML:

```toml
[app]
name = "William"
mention_username = "@william"

[openai]
model = "gpt-4o-mini"
temperature = 0.7
max_tokens_summarize = 2048

[prompts]
summarize_system = "You are a community secretary..."
response_system = "You are William, the community secretary..."
```

### Environment Variables

Secrets and deployment-specific settings:

```bash
# Required
TG_BOT_TOKEN=your_telegram_bot_token
OPENAI_API_KEY=your_openai_api_key
PG_DSN=postgres://user:pass@host:port/db

# Optional overrides
OPENAI_MODEL=gpt-4           # Override TOML setting
MAX_MSG_BUFFER=200           # Override TOML setting
APP_CONFIG_PATH=config/app.toml  # Custom config path
```

## Architecture

### Components

- **Bot Layer** (`internal/bot/`): Telegram event handling
- **Context Layer** (`internal/context/`): Message summarization and context building
- **GPT Client** (`internal/gpt/`): OpenAI GPT-4o integration
- **Database** (`internal/repo/`): PostgreSQL operations
- **Scheduler** (`internal/scheduler/`): Midnight cron jobs
- **Configuration** (`internal/config/`): TOML + ENV configuration

### Data Flow

1. **Message Ingestion**: Telegram → `listener.go` → Database + Counter
2. **Auto-Summarization**: N messages trigger → `summarizer.go` → GPT → Database
3. **Mention Handling**: @william → `builder.go` → Context + GPT → Response
4. **Midnight Reset**: Cron → Summarize all chats → Reset counters

## Development

### Available Commands

```bash
make help                 # Show all available commands
make build               # Build the application
make run                 # Run the application
make test                # Run tests
make test-coverage       # Run tests with coverage
make lint                # Run linters
make check-imports       # Check for unauthorized imports
make check               # Run full check pipeline
make migrate-up          # Run database migrations
make migrate-create      # Create new migration
make docker-build        # Build Docker image
make docker-run          # Run Docker container
```

### Database Migrations

```bash
# Create new migration
make migrate-create name=add_new_feature

# Apply migrations
make migrate-up

# Check status
make migrate-status
```

## Technology Stack

- **Language**: Go 1.24+
- **Database**: PostgreSQL 15 with pgx/v5 driver
- **AI**: OpenAI GPT-4o (v1.8.2)
- **Telegram**: Telego v1.2.0
- **Events**: Watermill pub/sub
- **DI**: samber/do container
- **Configuration**: go-toml/v2
- **Migrations**: goose v3

## Dependencies

All dependencies are strictly controlled via `allowed-mods.txt`. See project rules for adding new dependencies.

## License

[Add your license here] 