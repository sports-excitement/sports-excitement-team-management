package utils

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"sports-excitement-team-management/src/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger     *log.Logger
	fileLogger *log.Logger
	loggerOnce sync.Once
)

// initLogger initializes the logging system with both console and file output
func initLogger() {
	loggerOnce.Do(func() {
		// Ensure log directory exists
		if config.AppConfig != nil && config.AppConfig.LogFilePath != "" {
			logDir := filepath.Dir(config.AppConfig.LogFilePath)
			if err := os.MkdirAll(logDir, 0755); err != nil {
				log.Printf("Failed to create log directory %s: %v", logDir, err)
				// Fallback to console only logging
				logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
				return
			}

			// Verify we can write to the log file
			testFile, err := os.OpenFile(config.AppConfig.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Printf("Failed to access log file %s: %v", config.AppConfig.LogFilePath, err)
				// Fallback to console only logging
				logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
				return
			}
			testFile.Close()

			// Set up log rotation with lumberjack
			rotateWriter := &lumberjack.Logger{
				Filename:   config.AppConfig.LogFilePath,
				MaxSize:    config.AppConfig.LogMaxSize,    // megabytes
				MaxBackups: config.AppConfig.LogMaxBackups, // number of backups
				MaxAge:     config.AppConfig.LogMaxAge,     // days
				Compress:   true,                           // compress rotated files
			}

			// Console logger (existing behavior)
			logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)

			// File logger with rotation
			fileLogger = log.New(rotateWriter, "", log.LstdFlags|log.Lshortfile)

			// Also log to both console and file
			multiWriter := io.MultiWriter(os.Stdout, rotateWriter)
			logger = log.New(multiWriter, "", log.LstdFlags|log.Lshortfile)

			log.Printf("Log system initialized - File: %s, Directory: %s", config.AppConfig.LogFilePath, logDir)
		} else {
			// Fallback to console only
			logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
			log.Printf("Log system initialized with console output only")
		}
	})
}

// ensureLogger makes sure the logger is initialized
func ensureLogger() {
	if logger == nil {
		initLogger()
	}
}

// LogInfo logs informational messages (always shown)
func LogInfo(format string, args ...interface{}) {
	ensureLogger()
	logger.Printf("[INFO] "+format, args...)
}

// LogError logs error messages (always shown)
func LogError(format string, args ...interface{}) {
	ensureLogger()
	logger.Printf("[ERROR] "+format, args...)
}

// LogFatal logs fatal messages and exits (always shown)
func LogFatal(format string, args ...interface{}) {
	ensureLogger()
	logger.Fatalf("[FATAL] "+format, args...)
}

// LogVerbose logs verbose/debug messages (only when verbose logging is enabled)
func LogVerbose(format string, args ...interface{}) {
	if config.AppConfig != nil && config.AppConfig.EnableVerboseLogs {
		ensureLogger()
		logger.Printf("[VERBOSE] "+format, args...)
	}
}

// LogDebug is an alias for LogVerbose for clarity
func LogDebug(format string, args ...interface{}) {
	LogVerbose(format, args...)
}

// IsVerboseEnabled returns whether verbose logging is enabled
func IsVerboseEnabled() bool {
	return config.AppConfig != nil && config.AppConfig.EnableVerboseLogs
}

// RotateLogs manually triggers log rotation
func RotateLogs() error {
	if fileLogger != nil {
		if rotateWriter, ok := fileLogger.Writer().(*lumberjack.Logger); ok {
			return rotateWriter.Rotate()
		}
	}
	return nil
}

// CleanupLogs removes old log files based on configuration
func CleanupLogs() {
	if config.AppConfig != nil && config.AppConfig.LogFilePath != "" {
		logDir := filepath.Dir(config.AppConfig.LogFilePath)

		// Let lumberjack handle cleanup automatically based on MaxAge and MaxBackups
		LogVerbose("Log cleanup handled automatically by rotation policy")
		LogVerbose("Log directory: %s, Max age: %d days, Max backups: %d",
			logDir, config.AppConfig.LogMaxAge, config.AppConfig.LogMaxBackups)
	}
}

// GetLogStats returns current log file statistics
func GetLogStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if config.AppConfig != nil && config.AppConfig.LogFilePath != "" {
		if fileInfo, err := os.Stat(config.AppConfig.LogFilePath); err == nil {
			stats["current_size_bytes"] = fileInfo.Size()
			stats["current_size_mb"] = float64(fileInfo.Size()) / (1024 * 1024)
			stats["max_size_mb"] = config.AppConfig.LogMaxSize
			stats["max_backups"] = config.AppConfig.LogMaxBackups
			stats["max_age_days"] = config.AppConfig.LogMaxAge
			stats["log_file_path"] = config.AppConfig.LogFilePath
		} else {
			stats["error"] = err.Error()
		}
	}

	return stats
}
