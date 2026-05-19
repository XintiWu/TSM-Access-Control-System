package config

import (
	"os"
	"strings"
)

type Config struct {
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroup   string
	DBDSN        string
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
	group := os.Getenv("KAFKA_GROUP")
	if group == "" {
		group = "aggregation-workers"
	}
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "access:access@tcp(localhost:3306)/access_control?parseTime=true"
	}
	return Config{
		KafkaBrokers: strings.Split(brokers, ","),
		KafkaTopic:   topic,
		KafkaGroup:   group,
		DBDSN:        dsn,
	}
}
