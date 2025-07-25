package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// AppConfig holds application settings from TOML file
type AppConfig struct {
	App struct {
		Name            string  `toml:"name"`
		Description     string  `toml:"description"`
		MentionUsername string  `toml:"mention_username"`
		DefaultResponse string  `toml:"default_response"`
		AllowedChats    []int64 `toml:"allowed_chats"`
	} `toml:"app"`

	OpenAI struct {
		Model              string  `toml:"model"`
		Temperature        float64 `toml:"temperature"`
		MaxTokensSummarize int     `toml:"max_tokens_summarize"`
		MaxTokensResponse  int     `toml:"max_tokens_response"`
	} `toml:"openai"`

	Limits struct {
		MaxMsgBuffer         int `toml:"max_msg_buffer"`
		CtxMaxTokens         int `toml:"ctx_max_tokens"`
		RecentMessagesLimit  int `toml:"recent_messages_limit"`
		SummarizeMaxMessages int `toml:"summarize_max_messages"`
	} `toml:"limits"`

	Scheduler struct {
		CheckIntervalMinutes int    `toml:"check_interval_minutes"`
		Timezone             string `toml:"timezone"`
	} `toml:"scheduler"`

	Prompts struct {
		SummarizeSystem string `toml:"summarize_system"`
		ResponseSystem  string `toml:"response_system"`
	} `toml:"prompts"`
}

// Config holds all configuration for the application
type Config struct {
	// Environment variables (secrets)
	TelegramBotToken string
	OpenAIAPIKey     string
	PostgresDSN      string

	// Application settings from TOML
	App AppConfig

	// Derived fields
	Location *time.Location
}

// Load reads configuration from environment variables and TOML file
func Load() (*Config, error) {
	// Load app configuration from TOML file
	appCfg, err := loadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load app config: %w", err)
	}

	cfg := &Config{
		TelegramBotToken: os.Getenv("TG_BOT_TOKEN"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		PostgresDSN:      os.Getenv("PG_DSN"),
		App:              *appCfg,
	}

	// Allow environment variable overrides for some settings
	if envModel := os.Getenv("OPENAI_MODEL"); envModel != "" {
		cfg.App.OpenAI.Model = envModel
	}

	if envBufferStr := os.Getenv("MAX_MSG_BUFFER"); envBufferStr != "" {
		if maxMsgBuffer, err := strconv.Atoi(envBufferStr); err == nil {
			cfg.App.Limits.MaxMsgBuffer = maxMsgBuffer
		}
	}

	if envTokensStr := os.Getenv("CTX_MAX_TOKENS"); envTokensStr != "" {
		if ctxMaxTokens, err := strconv.Atoi(envTokensStr); err == nil {
			cfg.App.Limits.CtxMaxTokens = ctxMaxTokens
		}
	}

	if envTZ := os.Getenv("TZ"); envTZ != "" {
		cfg.App.Scheduler.Timezone = envTZ
	}

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
	location, err := time.LoadLocation(cfg.App.Scheduler.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %s: %w", cfg.App.Scheduler.Timezone, err)
	}
	cfg.Location = location

	return cfg, nil
}

// loadAppConfig loads application configuration from TOML file
func loadAppConfig() (*AppConfig, error) {
	configPath := getEnvWithDefault("APP_CONFIG_PATH", "config/app.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config AppConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse TOML config: %w", err)
	}

	return &config, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// IsChatAllowed checks if a chat ID is in the allowed chats list
// If AllowedChats is empty, all chats are allowed
func (c *Config) IsChatAllowed(chatID int64) bool {
	// If no restrictions are set, allow all chats
	if len(c.App.App.AllowedChats) == 0 {
		return true
	}

	// Check if chatID is in the allowed list
	for _, allowedID := range c.App.App.AllowedChats {
		if allowedID == chatID {
			return true
		}
	}

	return false
}
