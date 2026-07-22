package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCPort     string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
	RedisAddr    string
	RedisPass    string
	KafkaBrokers []string
	KafkaTopic   string
}

func Load() *Config {
	brokerEnv := os.Getenv("KAFKA_BROKER")
	var brokers []string
	if brokerEnv != "" && brokerEnv != "disabled" {
		brokers = []string{brokerEnv}
	} else if os.Getenv("RENDER") == "" {
		// Default to local broker only if running locally on machine
		brokers = []string{"localhost:9092"}
	}

	return &Config{
		GRPCPort:     getEnv("GRPC_PORT", "50051"),
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "postgres"),
		DBPassword:   getEnv("DB_PASSWORD", "postgres"),
		DBName:       getEnv("DB_NAME", "fx_settlement"),
		DBSSLMode:    getEnv("DB_SSLMODE", "disable"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:    getEnv("REDIS_PASSWORD", ""),
		KafkaBrokers: brokers,
		KafkaTopic:   getEnv("KAFKA_TOPIC", "settlement-audit-events"),
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
