package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	TelegramBotToken string
	OpenAIAPIKey     string
	OpenAIModel      string
	MaxMsgBuffer     int
	CtxMaxTokens     int
	PostgresDSN      string
	Timezone         string

	// Derived fields
	Location *time.Location
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		TelegramBotToken: os.Getenv("TG_BOT_TOKEN"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:      getEnvWithDefault("OPENAI_MODEL", "gpt-4o-mini"),
		PostgresDSN:      os.Getenv("PG_DSN"),
		Timezone:         getEnvWithDefault("TZ", "Europe/Belgrade"),
	}

	// Parse MaxMsgBuffer
	maxMsgBufferStr := getEnvWithDefault("MAX_MSG_BUFFER", "100")
	maxMsgBuffer, err := strconv.Atoi(maxMsgBufferStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_MSG_BUFFER: %w", err)
	}
	cfg.MaxMsgBuffer = maxMsgBuffer

	// Parse CtxMaxTokens
	ctxMaxTokensStr := getEnvWithDefault("CTX_MAX_TOKENS", "2048")
	ctxMaxTokens, err := strconv.Atoi(ctxMaxTokensStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CTX_MAX_TOKENS: %w", err)
	}
	cfg.CtxMaxTokens = ctxMaxTokens

	// Validate required fields
	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TG_BOT_TOKEN is required")
	}
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("PG_DSN is required")
	}

	// Parse timezone
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %s: %w", cfg.Timezone, err)
	}
	cfg.Location = location

	return cfg, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
