package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort           string
	MongoURI          string
	MongoDatabase     string
	JWTSecret         string
	JWTExpire         time.Duration
	UserCountInterval time.Duration
}

func Load() (*Config, error) {
	// ไม่มีไฟล์ .env ไม่ใช่ error — ตอนรันใน Docker จะใช้ env จริงจาก compose
	_ = godotenv.Load()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return &Config{
		AppPort:           getEnv("APP_PORT", "8080"),
		MongoURI:          getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:     getEnv("MONGO_DB", "userdb"),
		JWTSecret:         secret,
		JWTExpire:         time.Duration(getEnvInt("JWT_EXPIRE_MINUTES", 60)) * time.Minute,
		UserCountInterval: time.Duration(getEnvInt("USER_COUNT_INTERVAL_SECONDS", 10)) * time.Second,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}
