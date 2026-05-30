package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant environment variables
	os.Unsetenv("HTTP_ADDR")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("CLICKHOUSE_ADDR")
	os.Unsetenv("CLICKHOUSE_USER")
	os.Unsetenv("CLICKHOUSE_PASSWORD")
	os.Unsetenv("EXPORT_DIR")
	os.Unsetenv("API_KEY")
	os.Unsetenv("RATE_LIMIT_RPS")

	cfg := Load()

	assert.Equal(t, ":8082", cfg.HTTPAddr)
	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
	assert.Equal(t, "localhost:9000", cfg.ClickHouseAddr)
	assert.Equal(t, "default", cfg.ClickHouseUser)
	assert.Equal(t, "", cfg.ClickHousePass)
	assert.Equal(t, "/app/exports", cfg.ExportDir)
	assert.Equal(t, "", cfg.APIKey)
	assert.Equal(t, 0, cfg.RateLimitRPS)
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("HTTP_ADDR", ":9999")
	os.Setenv("REDIS_ADDR", "redis:6379")
	os.Setenv("CLICKHOUSE_ADDR", "ch:9000")
	os.Setenv("CLICKHOUSE_USER", "admin")
	os.Setenv("CLICKHOUSE_PASSWORD", "pwd")
	os.Setenv("EXPORT_DIR", "/tmp/exports")
	os.Setenv("API_KEY", "test-api-key")
	os.Setenv("RATE_LIMIT_RPS", "150")

	defer func() {
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("CLICKHOUSE_ADDR")
		os.Unsetenv("CLICKHOUSE_USER")
		os.Unsetenv("CLICKHOUSE_PASSWORD")
		os.Unsetenv("EXPORT_DIR")
		os.Unsetenv("API_KEY")
		os.Unsetenv("RATE_LIMIT_RPS")
	}()

	cfg := Load()

	assert.Equal(t, ":9999", cfg.HTTPAddr)
	assert.Equal(t, "redis:6379", cfg.RedisAddr)
	assert.Equal(t, "ch:9000", cfg.ClickHouseAddr)
	assert.Equal(t, "admin", cfg.ClickHouseUser)
	assert.Equal(t, "pwd", cfg.ClickHousePass)
	assert.Equal(t, "/tmp/exports", cfg.ExportDir)
	assert.Equal(t, "test-api-key", cfg.APIKey)
	assert.Equal(t, 150, cfg.RateLimitRPS)
}
