package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	envs := []string{"HTTP_ADDR", "REDIS_ADDR", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD", "EXPORT_DIR", "API_KEY", "RATE_LIMIT_RPS"}
	saved := make(map[string]string)
	for _, e := range envs {
		saved[e] = os.Getenv(e)
		os.Unsetenv(e)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	cfg := Load()

	if cfg.HTTPAddr != ":8082" {
		t.Errorf("HTTPAddr = %q, want :8082", cfg.HTTPAddr)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q, want localhost:6379", cfg.RedisAddr)
	}
	if cfg.ClickHouseAddr != "localhost:9000" {
		t.Errorf("ClickHouseAddr = %q, want localhost:9000", cfg.ClickHouseAddr)
	}
	if cfg.ClickHouseUser != "default" {
		t.Errorf("ClickHouseUser = %q, want default", cfg.ClickHouseUser)
	}
	if cfg.ClickHousePass != "" {
		t.Errorf("ClickHousePass = %q, want empty", cfg.ClickHousePass)
	}
	if cfg.ExportDir != "/app/exports" {
		t.Errorf("ExportDir = %q, want /app/exports", cfg.ExportDir)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
	if cfg.RateLimitRPS != 0 {
		t.Errorf("RateLimitRPS = %d, want 0", cfg.RateLimitRPS)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("HTTP_ADDR", ":9082")
	os.Setenv("REDIS_ADDR", "redis:6380")
	os.Setenv("CLICKHOUSE_ADDR", "ch:9440")
	os.Setenv("CLICKHOUSE_USER", "admin")
	os.Setenv("CLICKHOUSE_PASSWORD", "secret")
	os.Setenv("EXPORT_DIR", "/tmp/exports")
	os.Setenv("API_KEY", "test-key")
	os.Setenv("RATE_LIMIT_RPS", "200")
	defer func() {
		for _, e := range []string{"HTTP_ADDR", "REDIS_ADDR", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD", "EXPORT_DIR", "API_KEY", "RATE_LIMIT_RPS"} {
			os.Unsetenv(e)
		}
	}()

	cfg := Load()

	if cfg.HTTPAddr != ":9082" {
		t.Errorf("HTTPAddr = %q, want :9082", cfg.HTTPAddr)
	}
	if cfg.RedisAddr != "redis:6380" {
		t.Errorf("RedisAddr = %q, want redis:6380", cfg.RedisAddr)
	}
	if cfg.ClickHouseAddr != "ch:9440" {
		t.Errorf("ClickHouseAddr = %q, want ch:9440", cfg.ClickHouseAddr)
	}
	if cfg.ClickHouseUser != "admin" {
		t.Errorf("ClickHouseUser = %q, want admin", cfg.ClickHouseUser)
	}
	if cfg.ClickHousePass != "secret" {
		t.Errorf("ClickHousePass = %q, want secret", cfg.ClickHousePass)
	}
	if cfg.ExportDir != "/tmp/exports" {
		t.Errorf("ExportDir = %q, want /tmp/exports", cfg.ExportDir)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want test-key", cfg.APIKey)
	}
	if cfg.RateLimitRPS != 200 {
		t.Errorf("RateLimitRPS = %d, want 200", cfg.RateLimitRPS)
	}
}

func TestLoadRateLimitRPS_InvalidValue(t *testing.T) {
	os.Setenv("RATE_LIMIT_RPS", "abc")
	defer os.Unsetenv("RATE_LIMIT_RPS")

	cfg := Load()
	if cfg.RateLimitRPS != 0 {
		t.Errorf("RateLimitRPS = %d, want 0 for invalid input", cfg.RateLimitRPS)
	}
}
