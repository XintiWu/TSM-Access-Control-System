package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Unset all env vars to test defaults
	envs := []string{"HTTP_ADDR", "REDIS_ADDR", "KAFKA_BROKERS", "KAFKA_TOPIC", "OUTBOX_DIR", "API_KEY", "RATE_LIMIT_RPS", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD"}
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

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q, want localhost:6379", cfg.RedisAddr)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Errorf("KafkaBrokers = %v, want [localhost:9092]", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "inout-events" {
		t.Errorf("KafkaTopic = %q, want inout-events", cfg.KafkaTopic)
	}
	if cfg.OutboxDir != "/data/outbox" {
		t.Errorf("OutboxDir = %q, want /data/outbox", cfg.OutboxDir)
	}
	if cfg.ClickHouseUser != "default" {
		t.Errorf("ClickHouseUser = %q, want default", cfg.ClickHouseUser)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
	if cfg.RateLimitRPS != 0 {
		t.Errorf("RateLimitRPS = %d, want 0", cfg.RateLimitRPS)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("HTTP_ADDR", ":9090")
	os.Setenv("REDIS_ADDR", "redis:6380")
	os.Setenv("KAFKA_BROKERS", "k1:9092,k2:9092")
	os.Setenv("KAFKA_TOPIC", "custom-topic")
	os.Setenv("OUTBOX_DIR", "/tmp/outbox")
	os.Setenv("CLICKHOUSE_ADDR", "ch:9000")
	os.Setenv("CLICKHOUSE_USER", "admin")
	os.Setenv("CLICKHOUSE_PASSWORD", "secret")
	os.Setenv("API_KEY", "my-key")
	os.Setenv("RATE_LIMIT_RPS", "100")
	defer func() {
		for _, e := range []string{"HTTP_ADDR", "REDIS_ADDR", "KAFKA_BROKERS", "KAFKA_TOPIC", "OUTBOX_DIR", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD", "API_KEY", "RATE_LIMIT_RPS"} {
			os.Unsetenv(e)
		}
	}()

	cfg := Load()

	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.RedisAddr != "redis:6380" {
		t.Errorf("RedisAddr = %q, want redis:6380", cfg.RedisAddr)
	}
	if len(cfg.KafkaBrokers) != 2 {
		t.Errorf("KafkaBrokers count = %d, want 2", len(cfg.KafkaBrokers))
	}
	if cfg.KafkaTopic != "custom-topic" {
		t.Errorf("KafkaTopic = %q, want custom-topic", cfg.KafkaTopic)
	}
	if cfg.ClickHouseAddr != "ch:9000" {
		t.Errorf("ClickHouseAddr = %q, want ch:9000", cfg.ClickHouseAddr)
	}
	if cfg.ClickHouseUser != "admin" {
		t.Errorf("ClickHouseUser = %q, want admin", cfg.ClickHouseUser)
	}
	if cfg.ClickHousePass != "secret" {
		t.Errorf("ClickHousePass = %q, want secret", cfg.ClickHousePass)
	}
	if cfg.APIKey != "my-key" {
		t.Errorf("APIKey = %q, want my-key", cfg.APIKey)
	}
	if cfg.RateLimitRPS != 100 {
		t.Errorf("RateLimitRPS = %d, want 100", cfg.RateLimitRPS)
	}
}

func TestLoadRateLimitRPS_InvalidValue(t *testing.T) {
	os.Setenv("RATE_LIMIT_RPS", "not-a-number")
	defer os.Unsetenv("RATE_LIMIT_RPS")

	cfg := Load()
	if cfg.RateLimitRPS != 0 {
		t.Errorf("RateLimitRPS = %d, want 0 for invalid input", cfg.RateLimitRPS)
	}
}
