package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	TelegramBotToken   string
	DatabaseURL        string
	OllamaBaseURL      string
	OllamaModel        string
	LogLevel           string
	LogDir             string
	ConversationWindow int
	MetricsAddr        string
	AllowedUserIDs     map[int64]struct{}

	// RAG configuration
	RAGEnabled          bool
	EmbeddingModel      string
	RAGRetrievedCount   int
	RAGRecentCount      int
	RAGMinSimilarity    float64
	RAGSemanticWeight   float64
	RAGKeywordWeight    float64
	RAGRecencyWeight    float64
	RAGRecencyDecayDays float64
}

// Load reads configuration from environment variables.
// It also tries to load a .env file if present (ignored if missing).
func Load() (*Config, error) {
	// Load .env if it exists; ignore error if file is absent
	_ = godotenv.Load()

	allowedUserIDs, err := parseUserIDs(os.Getenv("ALLOWED_USER_IDS"))
	if err != nil {
		return nil, fmt.Errorf("parsing ALLOWED_USER_IDS: %w", err)
	}

	cfg := &Config{
		TelegramBotToken:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		OllamaBaseURL:      getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:        getEnvOrDefault("OLLAMA_MODEL", "llama3.1"),
		LogLevel:           getEnvOrDefault("LOG_LEVEL", "info"),
		LogDir:             getEnvOrDefault("LOG_DIR", "logs"),
		ConversationWindow: getEnvIntOrDefault("CONVERSATION_WINDOW", 20),
		MetricsAddr:        getEnvOrDefault("METRICS_ADDR", ":9090"),
		AllowedUserIDs:     allowedUserIDs,

		// RAG configuration
		RAGEnabled:          getEnvBoolOrDefault("RAG_ENABLED", false),
		EmbeddingModel:      getEnvOrDefault("EMBEDDING_MODEL", "nomic-embed-text"),
		RAGRetrievedCount:   getEnvIntOrDefault("RAG_RETRIEVED_COUNT", 5),
		RAGRecentCount:      getEnvIntOrDefault("RAG_RECENT_COUNT", 10),
		RAGMinSimilarity:    getEnvFloatOrDefault("RAG_MIN_SIMILARITY", 0.5),
		RAGSemanticWeight:   getEnvFloatOrDefault("RAG_WEIGHT_SEMANTIC", 0.5),
		RAGKeywordWeight:    getEnvFloatOrDefault("RAG_WEIGHT_KEYWORD", 0.3),
		RAGRecencyWeight:    getEnvFloatOrDefault("RAG_WEIGHT_RECENCY", 0.2),
		RAGRecencyDecayDays: getEnvFloatOrDefault("RAG_RECENCY_DECAY_DAYS", 30.0),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.AllowedUserIDs) == 0 {
		return fmt.Errorf("ALLOWED_USER_IDS is required (comma-separated Telegram user IDs)")
	}
	return nil
}

func parseUserIDs(raw string) (map[int64]struct{}, error) {
	result := make(map[int64]struct{})
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID %q: %w", s, err)
		}
		result[id] = struct{}{}
	}
	return result, nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvIntOrDefault(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func getEnvFloatOrDefault(key string, defaultVal float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func getEnvBoolOrDefault(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}
