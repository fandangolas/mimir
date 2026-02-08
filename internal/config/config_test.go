package config

import (
	"os"
	"testing"
)

func TestLoad_missingRequiredFields(t *testing.T) {
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("DATABASE_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required config, got nil")
	}
}

func TestLoad_defaults(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("ALLOWED_USER_IDS", "123456789")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OllamaBaseURL != "http://localhost:11434" {
		t.Errorf("unexpected OllamaBaseURL: %s", cfg.OllamaBaseURL)
	}
	if cfg.OllamaModel != "llama3.1" {
		t.Errorf("unexpected OllamaModel: %s", cfg.OllamaModel)
	}
	if cfg.ConversationWindow != 20 {
		t.Errorf("unexpected ConversationWindow: %d", cfg.ConversationWindow)
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("unexpected MetricsAddr: %s", cfg.MetricsAddr)
	}
}
