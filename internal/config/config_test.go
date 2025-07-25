package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "valid config",
			env: map[string]string{
				"TG_BOT_TOKEN":   "test_bot_token",
				"OPENAI_API_KEY": "test_openai_key",
				"PG_DSN":         "postgres://localhost/test",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test_bot_token", cfg.TelegramBotToken)
				assert.Equal(t, "test_openai_key", cfg.OpenAIAPIKey)
				assert.Equal(t, "gpt-4o-mini", cfg.OpenAIModel)
				assert.Equal(t, 100, cfg.MaxMsgBuffer)
				assert.Equal(t, 2048, cfg.CtxMaxTokens)
				assert.Equal(t, "postgres://localhost/test", cfg.PostgresDSN)
				assert.Equal(t, "Europe/Belgrade", cfg.Timezone)
				assert.NotNil(t, cfg.Location)
			},
		},
		{
			name: "missing telegram token",
			env: map[string]string{
				"OPENAI_API_KEY": "test_openai_key",
				"PG_DSN":         "postgres://localhost/test",
			},
			wantErr: true,
		},
		{
			name: "missing openai key",
			env: map[string]string{
				"TG_BOT_TOKEN": "test_bot_token",
				"PG_DSN":       "postgres://localhost/test",
			},
			wantErr: true,
		},
		{
			name: "missing postgres dsn",
			env: map[string]string{
				"TG_BOT_TOKEN":   "test_bot_token",
				"OPENAI_API_KEY": "test_openai_key",
			},
			wantErr: true,
		},
		{
			name: "custom values",
			env: map[string]string{
				"TG_BOT_TOKEN":   "test_bot_token",
				"OPENAI_API_KEY": "test_openai_key",
				"PG_DSN":         "postgres://localhost/test",
				"OPENAI_MODEL":   "gpt-4",
				"MAX_MSG_BUFFER": "50",
				"CTX_MAX_TOKENS": "1024",
				"TZ":             "UTC",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "gpt-4", cfg.OpenAIModel)
				assert.Equal(t, 50, cfg.MaxMsgBuffer)
				assert.Equal(t, 1024, cfg.CtxMaxTokens)
				assert.Equal(t, "UTC", cfg.Timezone)
			},
		},
		{
			name: "invalid max msg buffer",
			env: map[string]string{
				"TG_BOT_TOKEN":   "test_bot_token",
				"OPENAI_API_KEY": "test_openai_key",
				"PG_DSN":         "postgres://localhost/test",
				"MAX_MSG_BUFFER": "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid ctx max tokens",
			env: map[string]string{
				"TG_BOT_TOKEN":   "test_bot_token",
				"OPENAI_API_KEY": "test_openai_key",
				"PG_DSN":         "postgres://localhost/test",
				"CTX_MAX_TOKENS": "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid timezone",
			env: map[string]string{
				"TG_BOT_TOKEN":   "test_bot_token",
				"OPENAI_API_KEY": "test_openai_key",
				"PG_DSN":         "postgres://localhost/test",
				"TZ":             "Invalid/Timezone",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment
			for key, value := range tt.env {
				os.Setenv(key, value)
			}

			cfg, err := Load()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.check != nil {
					tt.check(t, cfg)
				}
			}
		})
	}
}

func TestGetEnvWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "env exists",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "env_value",
			expected:     "env_value",
		},
		{
			name:         "env not exists",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the key first
			os.Unsetenv(tt.key)

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvWithDefault(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
