package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddress  string        `env:"HTTP_ADDRESS" env-default:":8083"`
	DBDSN        string        `env:"DB_DSN" env-required:"true"`
	DBMaxOpen    int           `env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	DBMaxIdle    int           `env:"DB_MAX_IDLE_CONNS" env-default:"10"`
	DBMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" env-default:"5m"`
	KafkaBrokers string        `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	KafkaClientID string       `env:"KAFKA_CLIENT_ID" env-default:"order-service"`
	KafkaTopicOrderCreated     string `env:"KAFKA_TOPIC_ORDER_CREATED" env-default:"order.created"`
	KafkaTopicOrderAssigned    string `env:"KAFKA_TOPIC_ORDER_ASSIGNED" env-default:"order.assigned"`
	KafkaTopicOrderCompleted   string `env:"KAFKA_TOPIC_ORDER_COMPLETED" env-default:"order.completed"`
	KafkaTopicOrderCancelled   string `env:"KAFKA_TOPIC_ORDER_CANCELLED" env-default:"order.cancelled"`
	KafkaTopicReviewCreated    string `env:"KAFKA_TOPIC_REVIEW_CREATED" env-default:"review.created"`
	KafkaTopicComplaintCreated string `env:"KAFKA_TOPIC_COMPLAINT_CREATED" env-default:"complaint.created"`
	Env          string        `env:"ENV" env-default:"development"`
	LogLevel     string        `env:"LOG_LEVEL" env-default:"info"`
	LogFormat    string        `env:"LOG_FORMAT" env-default:"json"`
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env")

	var cfg Config
	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) KafkaBrokersList() []string {
	if c.KafkaBrokers == "" {
		return []string{"localhost:9092"}
	}
	return splitAndTrim(c.KafkaBrokers)
}

func (c *Config) DBDSNValue() string {
	return c.DBDSN
}

func (c *Config) DBMaxOpenConns() int {
	return c.DBMaxOpen
}

func (c *Config) DBMaxIdleConns() int {
	return c.DBMaxIdle
}

func (c *Config) DBConnMaxLifetime() time.Duration {
	return c.DBMaxLifetime
}

func splitAndTrim(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == ',' {
			trimmed := trimSpace(current)
			if trimmed != "" {
				result = append(result, trimmed)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	trimmed := trimSpace(current)
	if trimmed != "" {
		result = append(result, trimmed)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
