package config

import (
	"os"
	"strings"
)

type Config struct {
	KafkaBrokers   []string
	KafkaTopic     string
	KafkaGroup     string
	DBDSN          string
	ClickHouseAddr string
	ClickHouseUser string
	ClickHousePass string
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
	chAddr := os.Getenv("CLICKHOUSE_ADDR")
	if chAddr == "" {
		chAddr = "localhost:9000"
	}
	chUser := os.Getenv("CLICKHOUSE_USER")
	if chUser == "" {
		chUser = "default"
	}
	chPass := os.Getenv("CLICKHOUSE_PASSWORD")
	return Config{
		KafkaBrokers:   strings.Split(brokers, ","),
		KafkaTopic:     topic,
		KafkaGroup:     group,
		DBDSN:          dsn,
		ClickHouseAddr: chAddr,
		ClickHouseUser: chUser,
		ClickHousePass: chPass,
	}
}

