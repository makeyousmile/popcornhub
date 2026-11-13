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
	MetricsToken        string
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPassword        string
	SMTPFrom            string
	AppBaseURL          string
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
	metricsToken := getEnv("METRICS_TOKEN", "")
	smtpHost := getEnv("SMTP_HOST", "")
	smtpPort := getEnv("SMTP_PORT", "587")
	smtpUser := getEnv("SMTP_USER", "")
	smtpPassword := getEnv("SMTP_PASSWORD", "")
	smtpFrom := getEnv("SMTP_FROM", "no-reply@popcornhub.local")
	appBaseURL := getEnv("APP_BASE_URL", "http://localhost:8085")

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
		MetricsToken:        metricsToken,
		SMTPHost:            smtpHost,
		SMTPPort:            smtpPort,
		SMTPUser:            smtpUser,
		SMTPPassword:        smtpPassword,
		SMTPFrom:            smtpFrom,
		AppBaseURL:          appBaseURL,
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
