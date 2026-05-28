package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr         string
	ClickHouseAddr   string
	ClickHouseUser   string
	ClickHousePass   string
	KafkaBrokers     []string
	KafkaTopic       string
	APIKey           string // optional — enables X-API-Key authentication
	RateLimitRPS     int    // per-IP rate limit (requests per second); 0 = disabled
}

func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
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
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "permission-events"
	}
	apiKey := os.Getenv("API_KEY")
	rateLimitRPS, _ := strconv.Atoi(os.Getenv("RATE_LIMIT_RPS"))
	return Config{
		HTTPAddr:       addr,
		ClickHouseAddr: chAddr,
		ClickHouseUser: chUser,
		ClickHousePass: chPass,
		KafkaBrokers:   strings.Split(brokers, ","),
		KafkaTopic:     topic,
		APIKey:         apiKey,
		RateLimitRPS:   rateLimitRPS,
	}
}

