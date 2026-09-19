package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	GinMode            string
	MongoURI           string
	MongoDBName        string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	JWTSecret          string
	JWTExpirationHours int
	AllowedOrigins     []string
	TrustedProxies     []string
}

func LoadConfig() *Config {
	// Attempt to load .env file if available
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	ginMode := getEnv("GIN_MODE", "debug")
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	mongoDBName := getEnv("MONGO_DB_NAME", "polling_app")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDBStr := getEnv("REDIS_DB", "0")
	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		redisDB = 0
	}

	var jwtSecret string
	rawJWTSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if ginMode == "release" {
		if len(rawJWTSecret) < 32 || rawJWTSecret == "default-insecure-secret-key-replace-in-production" {
			panic("Fatal error: in production (GIN_MODE=release), JWT_SECRET environment variable must be set to a secure string with at least 32 characters")
		}
		jwtSecret = rawJWTSecret
	} else {
		jwtSecret = getEnv("JWT_SECRET", "default-insecure-secret-key-replace-in-production")
	}

	jwtExpHoursStr := getEnv("JWT_EXPIRATION_HOURS", "72")
	jwtExpHours, err := strconv.Atoi(jwtExpHoursStr)
	if err != nil {
		jwtExpHours = 72
	}

	allowedOriginsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")
	var allowedOrigins []string
	for _, origin := range strings.Split(allowedOriginsStr, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowedOrigins = append(allowedOrigins, trimmed)
		}
	}

	trustedProxiesStr := getEnv("TRUSTED_PROXIES", "")
	var trustedProxies []string
	if trustedProxiesStr != "" {
		for _, p := range strings.Split(trustedProxiesStr, ",") {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				trustedProxies = append(trustedProxies, trimmed)
			}
		}
	}

	return &Config{
		Port:               port,
		GinMode:            ginMode,
		MongoURI:           mongoURI,
		MongoDBName:        mongoDBName,
		RedisAddr:          redisAddr,
		RedisPassword:      redisPassword,
		RedisDB:            redisDB,
		JWTSecret:          jwtSecret,
		JWTExpirationHours: jwtExpHours,
		AllowedOrigins:     allowedOrigins,
		TrustedProxies:     trustedProxies,
	}
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}
