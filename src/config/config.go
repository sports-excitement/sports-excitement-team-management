package config

import (
	"os"
	"strconv"
)

type Config struct {
	SlackAppToken      string
	SlackBotToken      string
	AdminUser          string
	AdminPass          string
	TurnstileSiteKey   string
	TurnstileSecretKey string
	DatabasePath       string
	Port               string
	SessionKey         string
	EnableVerboseLogs  bool
	LogFilePath        string
	LogMaxSize         int    // Maximum size in MB before rotation
	LogMaxBackups      int    // Maximum number of backup files to keep
	LogMaxAge          int    // Maximum number of days to retain logs
}

var AppConfig *Config

func Init() {
	AppConfig = &Config{
		SlackAppToken:      os.Getenv("SLACK_APP_TOKEN"),
		SlackBotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		AdminUser:          getEnvOrDefault("ADMIN_USER", "admin"),
		AdminPass:          getEnvOrDefault("ADMIN_PASS", "admin"),
		TurnstileSiteKey:   os.Getenv("TURNSTILE_SITE_KEY"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		DatabasePath:       getEnvOrDefault("DATABASE_PATH", "./time_tracker.db"),
		Port:               getEnvOrDefault("PORT", "3000"),
		SessionKey:         getEnvOrDefault("SESSION_KEY", "your-secret-session-key"),
		EnableVerboseLogs:  getBoolEnv("ENABLE_VERBOSE_LOGS", true),
		LogFilePath:        getEnvOrDefault("LOG_FILE_PATH", "./data/tracker.log"),
		LogMaxSize:         GetIntEnv("LOG_MAX_SIZE_MB", 10),
		LogMaxBackups:      GetIntEnv("LOG_MAX_BACKUPS", 5),
		LogMaxAge:          GetIntEnv("LOG_MAX_AGE_DAYS", 30),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
} 