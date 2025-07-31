package main

import (
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"

	"sports-excitement-team-management/src/database"
	"sports-excitement-team-management/src/handlers"
	"sports-excitement-team-management/src/services"
	"sports-excitement-team-management/src/utils"
)

// ensureDataDirectories creates necessary directories if they don't exist
func ensureDataDirectories() {
	// Common directories that should exist
	directories := []string{
		"./data",   // Default data directory
		"./public", // Static files directory (should exist but just in case)
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			utils.LogError("Failed to create directory %s: %v", dir, err)
		}
	}

	// Also ensure any directory from environment variables exists
	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		if dbDir := filepath.Dir(dbPath); dbDir != "." && dbDir != "" {
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				utils.LogError("Failed to create database directory %s: %v", dbDir, err)
			}
		}
	}

	if logPath := os.Getenv("LOG_FILE_PATH"); logPath != "" {
		if logDir := filepath.Dir(logPath); logDir != "." && logDir != "" {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				utils.LogError("Failed to create log directory %s: %v", logDir, err)
			}
		}
	}
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Note: We can't use utils.LogVerbose here yet as config isn't initialized
		// This is fine - in Docker, env vars will be set via environment
	}

	// Ensure necessary directories exist before initializing systems
	ensureDataDirectories()

	// Initialize logging system early
	utils.LogInfo("Initializing application...")
	utils.CleanupLogs() // Log cleanup info

	// Initialize database
	database.Initialize()

	// Initialize template engine
	engine := html.New("./src/templates", ".html")
	engine.Reload(true) // for development

	// Create Fiber app
	app := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	// Static files
	app.Static("/", "./public")

	// Initialize WebSocket hub
	wsHub := services.NewWebSocketHub()
	services.SetGlobalHub(wsHub) // Set the global hub reference
	go wsHub.Run()

	// Initialize Slack service with initial status sync
	slackService := services.NewSlackService()
	slackService.StartWithInitialSync()

	// Initialize handlers
	handlers.SetupRoutes(app, wsHub)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	utils.LogInfo("Server starting on port %s", port)
	utils.LogInfo("Log configuration: %+v", utils.GetLogStats())
	utils.LogFatal(app.Listen(":" + port).Error())
}
