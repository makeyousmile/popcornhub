package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                string
	MongoURI            string
	MongoDBName         string
	RedisAddr           string
	RedisPassword       string
	JWTSecret           string
	JWTAccessExpiry     time.Duration
	JWTRefreshExpiry    time.Duration
	RateLimitIntervalMs int
	LogLevel            string
}

func Load() *Config {
	port := getEnv("PORT", "8085")
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	mongoDBName := getEnv("MONGO_DB", "popcornhub")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	jwtSecret := getEnv("JWT_SECRET", "super-secret-popcorn-key-change-in-prod-2026")
	rateLimitMs := getEnvInt("RATE_LIMIT_INTERVAL_MS", 1500)
	logLevel := getEnv("LOG_LEVEL", "info")

	return &Config{
		Port:                port,
		MongoURI:            mongoURI,
		MongoDBName:         mongoDBName,
		RedisAddr:           redisAddr,
		RedisPassword:       redisPassword,
		JWTSecret:           jwtSecret,
		JWTAccessExpiry:     15 * time.Minute,
		JWTRefreshExpiry:    7 * 24 * time.Hour,
		RateLimitIntervalMs: rateLimitMs,
		LogLevel:            logLevel,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
