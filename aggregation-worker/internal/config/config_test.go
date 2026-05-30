package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	envs := []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD"}
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

	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Errorf("KafkaBrokers = %v, want [localhost:9092]", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "inout-events" {
		t.Errorf("KafkaTopic = %q, want inout-events", cfg.KafkaTopic)
	}
	if cfg.KafkaGroup != "aggregation-workers" {
		t.Errorf("KafkaGroup = %q, want aggregation-workers", cfg.KafkaGroup)
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
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("KAFKA_BROKERS", "b1:9092,b2:9092,b3:9092")
	os.Setenv("KAFKA_TOPIC", "custom-topic")
	os.Setenv("KAFKA_GROUP", "custom-group")
	os.Setenv("CLICKHOUSE_ADDR", "ch:9440")
	os.Setenv("CLICKHOUSE_USER", "admin")
	os.Setenv("CLICKHOUSE_PASSWORD", "pw123")
	defer func() {
		for _, e := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP", "CLICKHOUSE_ADDR", "CLICKHOUSE_USER", "CLICKHOUSE_PASSWORD"} {
			os.Unsetenv(e)
		}
	}()

	cfg := Load()

	if len(cfg.KafkaBrokers) != 3 {
		t.Errorf("KafkaBrokers count = %d, want 3", len(cfg.KafkaBrokers))
	}
	if cfg.KafkaTopic != "custom-topic" {
		t.Errorf("KafkaTopic = %q, want custom-topic", cfg.KafkaTopic)
	}
	if cfg.KafkaGroup != "custom-group" {
		t.Errorf("KafkaGroup = %q, want custom-group", cfg.KafkaGroup)
	}
	if cfg.ClickHouseAddr != "ch:9440" {
		t.Errorf("ClickHouseAddr = %q, want ch:9440", cfg.ClickHouseAddr)
	}
	if cfg.ClickHouseUser != "admin" {
		t.Errorf("ClickHouseUser = %q, want admin", cfg.ClickHouseUser)
	}
	if cfg.ClickHousePass != "pw123" {
		t.Errorf("ClickHousePass = %q, want pw123", cfg.ClickHousePass)
	}
}
