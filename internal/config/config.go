// Package config centraliza a leitura de variaveis de ambiente. Preferimos
// isso a espalhar os.Getenv pelo codigo todo porque deixa explicito, num
// unico lugar, tudo que o gateway precisa para subir.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort      string
	AdminToken    string
	OracleDSN     string
	UseMemoryStore bool // modo de desenvolvimento sem Oracle, usado tambem nos testes de integracao locais
	RedisAddr    string
	RedisPassword string
	RedisDB       int
	LogLevel      string
	MetricsPath   string
	WSBroadcastInterval time.Duration
}

func Load() Config {
	return Config{
		HTTPPort:            getEnv("HTTP_PORT", "8080"),
		AdminToken:           getEnv("ADMIN_TOKEN", "admin-secret-token"),
		OracleDSN:            getEnv("ORACLE_DSN", ""),
		UseMemoryStore:       getBool("USE_MEMORY_STORE", false),
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              getInt("REDIS_DB", 0),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		MetricsPath:          getEnv("METRICS_PATH", "/metrics"),
		WSBroadcastInterval:  getDuration("WS_BROADCAST_INTERVAL", time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
