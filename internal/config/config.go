package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   string
	InternalJWTSecret      string
	RiotAPIKey             string
	RiotAPITimeout         time.Duration
	RiotRateLimitPerSecond float64
	RiotRateLimitBurst     int
	RiotRateLimitPer2Min   int
	RedisURL               string
	CacheEnabled           bool
	CBThreshold            int
	CBTimeout              time.Duration
	CBCooldown             time.Duration
	LogLevel               string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	secret := os.Getenv("INTERNAL_JWT_SECRET")
	if secret == "" {
		log.Fatal("INTERNAL_JWT_SECRET is required")
	}

	apiKey := os.Getenv("RIOT_API_KEY")
	if apiKey == "" {
		log.Fatal("RIOT_API_KEY is required")
	}

	return &Config{
		Port:                   getEnv("PORT", "4444"),
		InternalJWTSecret:      secret,
		RiotAPIKey:             apiKey,
		RiotAPITimeout:         getDuration("RIOT_API_TIMEOUT", 5*time.Second),
		RiotRateLimitPerSecond: getFloat("RIOT_RATE_LIMIT_PER_SECOND", 20.0),
		RiotRateLimitBurst:     getInt("RIOT_RATE_LIMIT_BURST", 20),
		RiotRateLimitPer2Min:   getInt("RIOT_RATE_LIMIT_PER_2MIN", 100),
		RedisURL:               getEnv("REDIS_URL", "redis://localhost:6379/1"),
		CacheEnabled:           getBool("CACHE_ENABLED", true),
		CBThreshold:            getInt("CIRCUIT_BREAKER_THRESHOLD", 5),
		CBTimeout:              getDuration("CIRCUIT_BREAKER_TIMEOUT", 60*time.Second),
		CBCooldown:             getDuration("CIRCUIT_BREAKER_COOLDOWN", 30*time.Second),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func getBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		n, nerr := strconv.Atoi(v)
		if nerr != nil {
			return fallback
		}
		return time.Duration(n) * time.Second
	}
	return d
}
