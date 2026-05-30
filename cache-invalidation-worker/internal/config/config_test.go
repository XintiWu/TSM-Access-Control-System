package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	envs := []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP", "REDIS_ADDR"}
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
	if cfg.KafkaTopic != "permission-events" {
		t.Errorf("KafkaTopic = %q, want permission-events", cfg.KafkaTopic)
	}
	if cfg.KafkaGroup != "cache-invalidation-workers" {
		t.Errorf("KafkaGroup = %q, want cache-invalidation-workers", cfg.KafkaGroup)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q, want localhost:6379", cfg.RedisAddr)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("KAFKA_BROKERS", "b1:9092,b2:9092")
	os.Setenv("KAFKA_TOPIC", "custom-topic")
	os.Setenv("KAFKA_GROUP", "custom-group")
	os.Setenv("REDIS_ADDR", "redis:6380")
	defer func() {
		for _, e := range []string{"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP", "REDIS_ADDR"} {
			os.Unsetenv(e)
		}
	}()

	cfg := Load()

	if len(cfg.KafkaBrokers) != 2 {
		t.Errorf("KafkaBrokers count = %d, want 2", len(cfg.KafkaBrokers))
	}
	if cfg.KafkaTopic != "custom-topic" {
		t.Errorf("KafkaTopic = %q, want custom-topic", cfg.KafkaTopic)
	}
	if cfg.KafkaGroup != "custom-group" {
		t.Errorf("KafkaGroup = %q, want custom-group", cfg.KafkaGroup)
	}
	if cfg.RedisAddr != "redis:6380" {
		t.Errorf("RedisAddr = %q, want redis:6380", cfg.RedisAddr)
	}
}
