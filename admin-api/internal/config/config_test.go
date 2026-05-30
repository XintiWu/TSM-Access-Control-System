package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	envs := []string{"HTTP_ADDR", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD", "KAFKA_BROKERS", "KAFKA_TOPIC", "API_KEY", "RATE_LIMIT_RPS"}
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

	if cfg.HTTPAddr != ":8081" {
		t.Errorf("HTTPAddr = %q, want :8081", cfg.HTTPAddr)
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
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Errorf("KafkaBrokers = %v, want [localhost:9092]", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "permission-events" {
		t.Errorf("KafkaTopic = %q, want permission-events", cfg.KafkaTopic)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
	if cfg.RateLimitRPS != 0 {
		t.Errorf("RateLimitRPS = %d, want 0", cfg.RateLimitRPS)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("HTTP_ADDR", ":9091")
	os.Setenv("CLICKHOUSE_ADDR", "ch:9000")
	os.Setenv("CLICKHOUSE_USER", "admin")
	os.Setenv("CLICKHOUSE_PASSWORD", "secret")
	os.Setenv("KAFKA_BROKERS", "k1:9092,k2:9092")
	os.Setenv("KAFKA_TOPIC", "custom-topic")
	os.Setenv("API_KEY", "my-key")
	os.Setenv("RATE_LIMIT_RPS", "50")
	defer func() {
		for _, e := range []string{"HTTP_ADDR", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD", "KAFKA_BROKERS", "KAFKA_TOPIC", "API_KEY", "RATE_LIMIT_RPS"} {
			os.Unsetenv(e)
		}
	}()

	cfg := Load()

	if cfg.HTTPAddr != ":9091" {
		t.Errorf("HTTPAddr = %q, want :9091", cfg.HTTPAddr)
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
	if len(cfg.KafkaBrokers) != 2 {
		t.Errorf("KafkaBrokers count = %d, want 2", len(cfg.KafkaBrokers))
	}
	if cfg.KafkaTopic != "custom-topic" {
		t.Errorf("KafkaTopic = %q, want custom-topic", cfg.KafkaTopic)
	}
	if cfg.APIKey != "my-key" {
		t.Errorf("APIKey = %q, want my-key", cfg.APIKey)
	}
	if cfg.RateLimitRPS != 50 {
		t.Errorf("RateLimitRPS = %d, want 50", cfg.RateLimitRPS)
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
