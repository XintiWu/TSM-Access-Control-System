package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	DBDSN        string
	KafkaBrokers []string
	KafkaTopic   string
}

func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "access:access@tcp(localhost:3307)/access_control?parseTime=true"
	}
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "permission-events"
	}
	return Config{
		HTTPAddr:     addr,
		DBDSN:        dsn,
		KafkaBrokers: strings.Split(brokers, ","),
		KafkaTopic:   topic,
	}
}
