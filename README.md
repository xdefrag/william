# William - Telegram Community Secretary Bot

William is a Telegram bot powered by ChatGPT (gpt-4o) that serves as a secretary for Telegram communities. It listens to all messages, creates context summaries, and provides intelligent responses when mentioned.

## Features

- **Message Ingestion**: Listens to all messages in configured chats and stores them in PostgreSQL
- **Smart Summarization**: Every N messages (default 100) and daily at 00:00, creates compressed context snapshots for chats and users
- **Context-Aware Responses**: When mentioned (@william or reply), gathers relevant context and responds using ChatGPT
- **Session Management**: Closes dialogues exactly at 00:00 and starts fresh sessions on next interaction
- **Event-Driven Architecture**: Uses Watermill for event processing and message queuing

## Architecture

```
cmd/
└── william/          # Entry point
internal/
├── bot/              # Telegram event handling
│   ├── listener.go   # Message ingestion
│   ├── handlers.go   # Event handlers
│   └── events.go     # Event definitions
├── context/          # Context building and summarization
│   ├── builder.go    # Context builder for responses
│   └── summarizer.go # Message summarization
├── gpt/              # OpenAI wrapper
│   └── client.go     # GPT client
├── repo/             # Database access layer
│   └── repository.go # PostgreSQL operations
├── scheduler/        # Cron jobs
│   └── scheduler.go  # Midnight and N-message triggers
└── config/           # Configuration
    └── config.go     # Environment variables
pkg/
└── models/           # Data types
    └── message.go    # Message, ChatSummary, UserSummary
```

## Technology Stack

- **Language**: Go 1.24+
- **Telegram API**: Telego
- **LLM**: OpenAI GPT-4o (via oai-go)
- **Database**: PostgreSQL 15
- **PaaS**: Railway
- **Message Broker**: Watermill (in-process)
- **Utilities**: samber/lo, samber/do (DI)
- **Migrations**: goose
- **Logging**: Watermill logger

## Database Schema

```sql
-- Messages storage
CREATE TABLE messages (
  id              BIGSERIAL PRIMARY KEY,
  telegram_msg_id BIGINT NOT NULL,
  chat_id         BIGINT NOT NULL,
  user_id         BIGINT NOT NULL,
  text            TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);

-- Chat snapshots
CREATE TABLE chat_summaries (
  id              BIGSERIAL PRIMARY KEY,
  chat_id         BIGINT NOT NULL,
  summary         TEXT NOT NULL,
  topics_json     JSONB NOT NULL DEFAULT '{}',
  next_events     TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);

-- User snapshots
CREATE TABLE user_summaries (
  id              BIGSERIAL PRIMARY KEY,
  chat_id         BIGINT NOT NULL,
  user_id         BIGINT NOT NULL,
  likes_json      JSONB NOT NULL DEFAULT '{}',
  dislikes_json   JSONB NOT NULL DEFAULT '{}',
  traits          TEXT,
  created_at      TIMESTAMPTZ DEFAULT now()
);
```

## Configuration

Set the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TG_BOT_TOKEN` | - | Telegram bot token (required) |
| `OPENAI_API_KEY` | - | OpenAI API key (required) |
| `OPENAI_MODEL` | `gpt-4o-mini` | OpenAI model to use |
| `MAX_MSG_BUFFER` | `100` | Message threshold for auto-summarization |
| `CTX_MAX_TOKENS` | `2048` | Token limit for single request |
| `PG_DSN` | - | PostgreSQL connection string (required) |
| `TZ` | `Europe/Belgrade` | Timezone for midnight processing |

## Development Setup

1. **Install dependencies**:
   ```bash
   make setup-dev
   ```

2. **Set up environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Run database migrations**:
   ```bash
   make migrate-up
   ```

4. **Start development server**:
   ```bash
   make dev  # Hot reload
   # or
   make run  # Standard run
   ```

## Available Commands

```bash
make help           # Show all available commands
make build          # Build the application
make run            # Run the application
make test           # Run tests
make test-coverage  # Run tests with coverage
make lint           # Run linters
make check-imports  # Check for unauthorized imports
make migrate-up     # Run database migrations
make migrate-down   # Rollback migrations
make docker-build   # Build Docker image
make docker-run     # Run in Docker
make check          # Run all checks (lint, test, build)
```

## Deployment

### Railway

1. Connect your GitHub repository to Railway
2. Set environment variables in Railway dashboard
3. Railway will automatically deploy on push to main branch

### Docker

```bash
# Build image
make docker-build

# Run container
docker run --env-file .env william-bot
```

## Data Flow

### Message Ingestion
1. Telegram update → `listener.go`
2. Save message to database + increment counter
3. If counter ≥ `MAX_MSG_BUFFER` → trigger summarization

### Summarization
1. Fetch last N messages from chat
2. Send to GPT-4o with summarization prompt
3. Save chat summary and user profiles
4. Reset message counter

### Mention Handling
1. Detect @william mention or reply to bot
2. Build context: last chat summary + recent messages + user profile
3. Send to GPT-4o with context-aware prompt
4. Reply to user

### Midnight Reset
1. Cron job triggers at 00:00 (configured timezone)
2. Summarize all active chats
3. Reset all message counters
4. Clear session state

## API Cost Management

- Middleware logs `prompt_tokens`/`completion_tokens`
- In-memory cache for recent summaries (< 1h)
- Weekly cost reporting available
- Budget limit: ≤ $100/month

## Dependency Management

This project uses a **fixed set of dependencies** defined in `allowed-mods.txt`. Any new imports must be approved and added to this list. The CI checks enforce this restriction.

## Testing

```bash
make test           # Unit tests
make test-coverage  # Coverage report
```

Coverage types:
- Unit tests: config parsing, repository methods, context building
- Integration tests: Telegram webhook + PostgreSQL (test containers)
- E2E tests: Mock OpenAI, simulated chats

## Performance Targets

| Metric | Target |
|--------|--------|
| Response time to mention | < 2 sec (95th percentile) |
| Summary topic accuracy | ≥ 90% (manual sampling) |
| LLM budget | ≤ $100/month |

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run `make check` to ensure code quality
4. Submit a pull request

## License

This project is proprietary software developed for specific community management needs. 