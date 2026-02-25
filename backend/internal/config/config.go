package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port        string
	Environment string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// JWT
	JWTSecret     string
	JWTExpiry     time.Duration
	RefreshExpiry time.Duration
}

func Load() *Config {
	return &Config{
		// Server
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "qrapp"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// JWT
		JWTSecret:     getEnv("JWT_SECRET", "your-super-secret-key-change-in-production"),
		JWTExpiry:     getDurationEnv("JWT_EXPIRY_MINUTES", 15) * time.Minute,
		RefreshExpiry: getDurationEnv("REFRESH_EXPIRY_DAYS", 7) * 24 * time.Hour,
	}
}

func (c *Config) DatabaseURL() string {
	dsn := "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
	if c.DBPassword != "" {
		dsn += " password=" + c.DBPassword
	}
	return dsn
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue int) time.Duration {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return time.Duration(intVal)
		}
	}
	return time.Duration(defaultValue)
}
