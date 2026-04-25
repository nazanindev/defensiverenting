package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	ListenAddr  string
	LogLevel    string
	Env         string
}

func Load() Config {
	return Config{
		DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tenants?sslmode=disable"),
		ListenAddr:  env("LISTEN_ADDR", ":8080"),
		LogLevel:    env("LOG_LEVEL", "info"),
		Env:         env("ENV", "development"),
	}
}

func (c Config) IsDevelopment() bool {
	return c.Env == "development"
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
