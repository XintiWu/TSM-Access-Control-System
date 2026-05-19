package config

import (
	"os"
	"strings"
)

type Config struct {
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroup   string
	RedisAddr    string
}

func Load() Config {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "permission-events"
	}
	group := os.Getenv("KAFKA_GROUP")
	if group == "" {
		group = "cache-invalidation-workers"
	}
	redis := os.Getenv("REDIS_ADDR")
	if redis == "" {
		redis = "localhost:6379"
	}
	return Config{
		KafkaBrokers: strings.Split(brokers, ","),
		KafkaTopic:   topic,
		KafkaGroup:   group,
		RedisAddr:    redis,
	}
}
