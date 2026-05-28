package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the report-api service.
type Config struct {
	HTTPAddr       string
	RedisAddr      string
	ClickHouseAddr string
	ClickHouseUser string
	ClickHousePass string
	ExportDir      string
	APIKey         string // optional — enables X-API-Key authentication
	RateLimitRPS   int    // per-IP rate limit (requests per second); 0 = disabled
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8082"
	}
	redis := os.Getenv("REDIS_ADDR")
	if redis == "" {
		redis = "localhost:6379"
	}
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	chUser := os.Getenv("CLICKHOUSE_USER")
	if chUser == "" {
		chUser = "default"
	}
	chPass := os.Getenv("CLICKHOUSE_PASSWORD")
	exportDir := os.Getenv("EXPORT_DIR")
	if exportDir == "" {
		exportDir = "/app/exports"
	}
	apiKey := os.Getenv("API_KEY")
	rateLimitRPS, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_RPS"))
	return Config{
		HTTPAddr:       addr,
		RedisAddr:      redis,
		ClickHouseAddr: chAddr,
		ClickHouseUser: chUser,
		ClickHousePass: chPass,
		ExportDir:      exportDir,
		APIKey:         apiKey,
		RateLimitRPS:   rateLimitRPS,
	}
}

