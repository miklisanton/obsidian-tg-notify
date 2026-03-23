package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesProcessEnvWithoutDotEnvFile(t *testing.T) {
	t.Setenv("APP_ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("APP_TIMEZONE", "UTC")
	t.Setenv("POSTGRES_HOST", "db")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_DB", "notify")
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_ALLOWED_CHAT_ID", "42")
	t.Setenv("PERSONAL_VAULT_PATH", "/vault")

	configPath := writeConfigFile(t)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Postgres.Host != "db" {
		t.Fatalf("Postgres.Host = %q, want %q", cfg.Postgres.Host, "db")
	}
	if len(cfg.Telegram.AllowedChatIDs) != 1 || cfg.Telegram.AllowedChatIDs[0] != 42 {
		t.Fatalf("AllowedChatIDs = %v, want [42]", cfg.Telegram.AllowedChatIDs)
	}
	if len(cfg.Vaults) != 1 || cfg.Vaults[0].RootPath != "/vault" {
		t.Fatalf("Vaults = %+v, want root_path=/vault", cfg.Vaults)
	}
}

func TestLoadUsesCustomEnvFileWhenConfigured(t *testing.T) {
	t.Setenv("APP_ENV_FILE", filepath.Join(t.TempDir(), "custom.env"))
	envPath := os.Getenv("APP_ENV_FILE")

	envContent := []byte("APP_TIMEZONE=UTC\nPOSTGRES_HOST=env-db\nPOSTGRES_PORT=5433\nPOSTGRES_DB=notify\nPOSTGRES_USER=user\nPOSTGRES_PASSWORD=pass\nPOSTGRES_SSLMODE=require\nTELEGRAM_BOT_TOKEN=token\nTELEGRAM_ALLOWED_CHAT_ID=7\nPERSONAL_VAULT_PATH=/env-vault\n")
	if err := os.WriteFile(envPath, envContent, 0o600); err != nil {
		t.Fatalf("WriteFile env: %v", err)
	}

	configPath := writeConfigFile(t)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Postgres.Port != 5433 {
		t.Fatalf("Postgres.Port = %d, want %d", cfg.Postgres.Port, 5433)
	}
	if cfg.Postgres.SSLMode != "require" {
		t.Fatalf("Postgres.SSLMode = %q, want %q", cfg.Postgres.SSLMode, "require")
	}
	if len(cfg.Vaults) != 1 || cfg.Vaults[0].RootPath != "/env-vault" {
		t.Fatalf("Vaults = %+v, want root_path=/env-vault", cfg.Vaults)
	}
}

func writeConfigFile(t *testing.T) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := []byte("app:\n  timezone: \"${APP_TIMEZONE}\"\n\npostgres:\n  host: \"${POSTGRES_HOST}\"\n  port: ${POSTGRES_PORT}\n  name: \"${POSTGRES_DB}\"\n  user: \"${POSTGRES_USER}\"\n  password: \"${POSTGRES_PASSWORD}\"\n  sslmode: \"${POSTGRES_SSLMODE}\"\n\ntelegram:\n  token: \"${TELEGRAM_BOT_TOKEN}\"\n  allowed_chat_ids:\n    - ${TELEGRAM_ALLOWED_CHAT_ID}\n\nvaults:\n  - id: 1\n    name: \"personal\"\n    root_path: \"${PERSONAL_VAULT_PATH}\"\n")
	if err := os.WriteFile(configPath, configContent, 0o600); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}
	return configPath
}
