package config

import "os"

// Config holds all configuration for the report-api service.
type Config struct {
	HTTPAddr       string // default ":8082"
	RedisAddr      string // Report Cache
	DBDSN          string // MariaDB connection
	ClickHouseAddr string // ClickHouse Native TCP address
	ClickHouseUser string
	ClickHousePass string
	ExportDir      string
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
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "access:access@tcp(localhost:3306)/access_control?parseTime=true"
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
	return Config{
		HTTPAddr:       addr,
		RedisAddr:      redis,
		DBDSN:          dsn,
		ClickHouseAddr: chAddr,
		ClickHouseUser: chUser,
		ClickHousePass: chPass,
		ExportDir:      exportDir,
	}
}
