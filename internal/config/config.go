// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime settings.
type Config struct {
	Port      string
	JWTSecret string
	JWTTTL    time.Duration
}

// Load reads configuration from the environment, applying sensible defaults so
// the service runs locally with zero setup. JWT_SECRET should always be set in
// any real deployment.
func Load() Config {
	return Config{
		Port:      getEnv("PORT", "8080"),
		JWTSecret: getEnv("JWT_SECRET", "dev-insecure-secret-change-me"),
		JWTTTL:    time.Duration(getEnvInt("JWT_TTL_HOURS", 24)) * time.Hour,
	}
}

// UsingDefaultSecret reports whether the insecure development secret is in use,
// so main can emit a warning.
func (c Config) UsingDefaultSecret() bool {
	return c.JWTSecret == "dev-insecure-secret-change-me"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
