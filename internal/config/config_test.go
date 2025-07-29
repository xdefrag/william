package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("TG_BOT_TOKEN", "test_token")
	_ = os.Setenv("OPENAI_API_KEY", "test_api_key")
	_ = os.Setenv("PG_DSN", "test_dsn")
	_ = os.Setenv("JWT_SECRET", "test_jwt_secret")
	_ = os.Setenv("APP_CONFIG_PATH", "../../config/app.toml")

	defer func() {
		_ = os.Unsetenv("TG_BOT_TOKEN")
		_ = os.Unsetenv("OPENAI_API_KEY")
		_ = os.Unsetenv("PG_DSN")
		_ = os.Unsetenv("JWT_SECRET")
		_ = os.Unsetenv("APP_CONFIG_PATH")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test required fields
	if cfg.TelegramBotToken != "test_token" {
		t.Errorf("Expected TelegramBotToken to be 'test_token', got %s", cfg.TelegramBotToken)
	}
	if cfg.OpenAIAPIKey != "test_api_key" {
		t.Errorf("Expected OpenAIAPIKey to be 'test_api_key', got %s", cfg.OpenAIAPIKey)
	}
	if cfg.PostgresDSN != "test_dsn" {
		t.Errorf("Expected PostgresDSN to be 'test_dsn', got %s", cfg.PostgresDSN)
	}
	if cfg.JWTSecret != "test_jwt_secret" {
		t.Errorf("Expected JWTSecret to be 'test_jwt_secret', got %s", cfg.JWTSecret)
	}

	// Test TOML loaded values
	if cfg.App.OpenAI.Model != "gpt-4o-mini" {
		t.Errorf("Expected OpenAI model to be 'gpt-4o-mini', got %s", cfg.App.OpenAI.Model)
	}
	if cfg.App.Limits.MaxMsgBuffer != 25 {
		t.Errorf("Expected MaxMsgBuffer to be 25, got %d", cfg.App.Limits.MaxMsgBuffer)
	}
	if cfg.App.Limits.CtxMaxTokens != 2048 {
		t.Errorf("Expected CtxMaxTokens to be 2048, got %d", cfg.App.Limits.CtxMaxTokens)
	}
	if cfg.App.Scheduler.Timezone != "Europe/Belgrade" {
		t.Errorf("Expected Timezone to be 'Europe/Belgrade', got %s", cfg.App.Scheduler.Timezone)
	}

	// Test app settings
	if cfg.App.App.Name != "Лемур-тян" {
		t.Errorf("Expected app name to be 'Лемур-тян', got %s", cfg.App.App.Name)
	}
	if cfg.App.App.MentionUsername != "@lemurchan_bot" {
		t.Errorf("Expected mention username to be '@lemurchan_bot', got %s", cfg.App.App.MentionUsername)
	}

	// Test prompts
	if cfg.App.Prompts.SummarizeSystem == "" {
		t.Error("Expected SummarizeSystem to be non-empty")
	}
	if cfg.App.Prompts.ResponseSystem == "" {
		t.Error("Expected ResponseSystem to be non-empty")
	}

	// Test location
	if cfg.Location == nil {
		t.Error("Expected location to be parsed")
	}
}

func TestLoadWithEnvOverrides(t *testing.T) {
	// Set environment variables including overrides
	_ = os.Setenv("TG_BOT_TOKEN", "test_token")
	_ = os.Setenv("OPENAI_API_KEY", "test_api_key")
	_ = os.Setenv("PG_DSN", "test_dsn")
	_ = os.Setenv("JWT_SECRET", "test_jwt_secret")
	_ = os.Setenv("APP_CONFIG_PATH", "../../config/app.toml")
	_ = os.Setenv("OPENAI_MODEL", "gpt-4")
	_ = os.Setenv("MAX_MSG_BUFFER", "200")
	_ = os.Setenv("CTX_MAX_TOKENS", "4096")
	_ = os.Setenv("TZ", "UTC")

	defer func() {
		_ = os.Unsetenv("TG_BOT_TOKEN")
		_ = os.Unsetenv("OPENAI_API_KEY")
		_ = os.Unsetenv("PG_DSN")
		_ = os.Unsetenv("JWT_SECRET")
		_ = os.Unsetenv("APP_CONFIG_PATH")
		_ = os.Unsetenv("OPENAI_MODEL")
		_ = os.Unsetenv("MAX_MSG_BUFFER")
		_ = os.Unsetenv("CTX_MAX_TOKENS")
		_ = os.Unsetenv("TZ")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test environment overrides
	if cfg.App.OpenAI.Model != "gpt-4" {
		t.Errorf("Expected OpenAI model to be 'gpt-4' (env override), got %s", cfg.App.OpenAI.Model)
	}
	if cfg.App.Limits.MaxMsgBuffer != 200 {
		t.Errorf("Expected MaxMsgBuffer to be 200 (env override), got %d", cfg.App.Limits.MaxMsgBuffer)
	}
	if cfg.App.Limits.CtxMaxTokens != 4096 {
		t.Errorf("Expected CtxMaxTokens to be 4096 (env override), got %d", cfg.App.Limits.CtxMaxTokens)
	}
	if cfg.App.Scheduler.Timezone != "UTC" {
		t.Errorf("Expected Timezone to be 'UTC' (env override), got %s", cfg.App.Scheduler.Timezone)
	}
}

func TestLoadMissingRequiredEnv(t *testing.T) {
	// Clear any existing environment variables
	_ = os.Unsetenv("TG_BOT_TOKEN")
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("PG_DSN")

	_, err := Load()
	if err == nil {
		t.Error("Expected error when required environment variables are missing")
	}
}
