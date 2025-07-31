package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"sports-excitement-team-management/src/services"
)

// SetupRoutes configures all application routes
func SetupRoutes(app *fiber.App, wsHub *services.WebSocketHub) {
	// Initialize session
	initSession()

	// Public routes (no auth required)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/login")
	})

	// Auth routes (apply AuthMiddleware selectively)
	app.Get("/login", AuthMiddleware, ShowLogin)
	app.Post("/login", AuthMiddleware, HandleLogin)
	app.Get("/logout", AuthMiddleware, HandleLogout)

	// WebSocket route with authentication check
	app.Use("/ws", func(c *fiber.Ctx) error {
		// Check if request is websocket upgrade
		if websocket.IsWebSocketUpgrade(c) {
			// Check authentication via session cookie
			if store == nil {
				initSession()
			}
			
			sess, err := store.Get(c)
			if err != nil || sess.Get("authenticated") != true {
				return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
			}
			
			c.Locals("allowed", true)
			c.Locals("user_id", sess.Get("user_id"))
			c.Locals("username", sess.Get("username"))
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		HandleWebSocket(c)
	}))

	// Protected routes (require authentication)
	protected := app.Group("/", RequireAuth)

	// Dashboard routes
	protected.Get("/dashboard", ShowDashboard)

	// API routes
	protected.Get("/api/users", GetUsersAPI)
	protected.Get("/api/analytics", GetAnalyticsAPI)
	protected.Get("/api/reports/weekly", GetWeeklyReports)
	
	// Log management API routes
	protected.Get("/api/logs/stats", GetLogStatsAPI)
	protected.Post("/api/logs/rotate", RotateLogsAPI)
} 