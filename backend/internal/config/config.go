package config

import (
	"os"
	"strconv"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     int
	RedisPassword string

	// JWT
	JWTSecret     string
	JWTExpireHours int

	// API
	APIPort int

	// RADIUS
	RadiusAuthPort int
	RadiusAcctPort int
	RadiusSecret   string

	// SaaS
	SaaSMode       bool
	AdminDBSchema  string
}

func Load() *Config {
	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "proisp"),
		DBPassword: getEnv("DB_PASSWORD", "ProISP@2024Secure!"),
		DBName:     getEnv("DB_NAME", "proxpanel"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnvInt("REDIS_PORT", 6379),
		RedisPassword: getEnv("REDIS_PASSWORD", "ProISP@Redis2024!"),

		// JWT
		JWTSecret:      getEnv("JWT_SECRET", "ProISP-JWT-Secret-Key-2024-Very-Secure!"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 168), // 7 days default

		// API
		APIPort: getEnvInt("API_PORT", 8080),

		// RADIUS
		RadiusAuthPort: getEnvInt("RADIUS_AUTH_PORT", 1812),
		RadiusAcctPort: getEnvInt("RADIUS_ACCT_PORT", 1813),
		RadiusSecret:   getEnv("RADIUS_SECRET", "radiussecret"),

		// SaaS
		SaaSMode:      getEnv("SAAS_MODE", "false") == "true",
		AdminDBSchema: getEnv("ADMIN_DB_SCHEMA", "admin"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
