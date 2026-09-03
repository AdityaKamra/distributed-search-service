package config

import "os"

type Config struct {
	Port         string
	PostgresDSN  string
	RedisAddr    string
	ESURL        string
	KafkaBrokers string
}

func LoadConfig() *Config {
	return &Config{
		Port:         getEnv("APP_PORT", "8080"),
		PostgresDSN:  getEnv("POSTGRES_DSN", "postgres://search_admin:admin_password_123@localhost:5432/search_service?sslmode=disable"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		ESURL:        getEnv("ES_URL", "http://localhost:9200"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
