package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	RedisAddr    string
	KafkaBrokers []string
	KafkaTopic   string
	DBDSN        string // optional — enables fallback when Redis is down
}

func Load() Config {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "inout-events"
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	redis := os.Getenv("REDIS_ADDR")
	if redis == "" {
		redis = "localhost:6379"
	}
	dbDSN := os.Getenv("DB_DSN") // empty = no DB fallback
	return Config{
		HTTPAddr:     addr,
		RedisAddr:    redis,
		KafkaBrokers: strings.Split(brokers, ","),
		KafkaTopic:   topic,
		DBDSN:        dbDSN,
	}
}

