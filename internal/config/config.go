package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	SessionSecret      string

	// AI configuration
	OpenAIAPIKey      string
	OpenAIBaseURL     string
	OpenAIModel       string
	AIWorkerPoolSize  int
	AIRetryInterval   time.Duration
	AIEnabled         bool
}

func Load() (*Config, error) {
	// .env is optional; in production, env vars are set directly.
	_ = godotenv.Load()

	poolSize, _ := strconv.Atoi(getEnv("AI_WORKER_POOL_SIZE", "1"))
	if poolSize < 1 {
		poolSize = 1
	}

	retryInterval, err := time.ParseDuration(getEnv("AI_RETRY_INTERVAL", "1h"))
	if err != nil {
		retryInterval = time.Hour
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	aiEnabled := apiKey != ""
	if !aiEnabled {
		log.Println("WARNING: OPENAI_API_KEY not set — AI summary features are disabled")
	}

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://teamback:teamback@localhost:5432/teamback?sslmode=disable"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),

		OpenAIAPIKey:      apiKey,
		OpenAIBaseURL:     getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:       getEnv("OPENAI_MODEL", "gpt-5-mini"),
		AIWorkerPoolSize:  poolSize,
		AIRetryInterval:   retryInterval,
		AIEnabled:         aiEnabled,
	}

	if cfg.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if cfg.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
