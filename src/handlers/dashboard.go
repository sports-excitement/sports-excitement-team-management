package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"sports-excitement-team-management/src/database"
	"sports-excitement-team-management/src/services"
	"sports-excitement-team-management/src/utils"
)

// ShowDashboard displays the main dashboard
func ShowDashboard(c *fiber.Ctx) error {
	// Get user summaries
	summaries, err := database.GetUserSummaries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load user data",
		})
	}

	// Calculate analytics
	analytics := calculateAnalytics(summaries)

	return c.Render("dashboard/index", fiber.Map{
		"Title":     "Time Tracker Dashboard",
		"Users":     summaries,
		"Analytics": analytics,
		"Username":  c.Locals("username"),
	})
}

// GetUsersAPI returns user data as JSON for API calls
func GetUsersAPI(c *fiber.Ctx) error {
	summaries, err := database.GetUserSummaries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load user data",
		})
	}

	return c.JSON(fiber.Map{
		"users": summaries,
	})
}

// GetAnalyticsAPI returns analytics data as JSON
func GetAnalyticsAPI(c *fiber.Ctx) error {
	summaries, err := database.GetUserSummaries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load analytics data",
		})
	}

	analytics := calculateAnalytics(summaries)

	return c.JSON(analytics)
}

// GetWeeklyReports returns weekly time tracking reports
func GetWeeklyReports(c *fiber.Ctx) error {
	// Parse week parameter (optional)
	weekParam := c.Query("week")
	var weekStart time.Time
	var err error

	if weekParam != "" {
		weekStart, err = time.Parse("2006-01-02", weekParam)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid week format. Use YYYY-MM-DD",
			})
		}
	} else {
		// Default to current week (Monday)
		now := time.Now()
		weekStart = now.AddDate(0, 0, -int(now.Weekday())+1)
	}

	// Ensure weekStart is Monday
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}

	reports, err := database.GetWeeklyReports(weekStart)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load weekly reports",
		})
	}

	return c.JSON(fiber.Map{
		"reports":    reports,
		"week_start": weekStart.Format("2006-01-02"),
		"week_end":   weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
	})
}

// ExportExcel generates Excel export for user data
func ExportExcel(c *fiber.Ctx) error {
	reportType := c.Query("type", "users")
	
	switch reportType {
	case "users":
		return exportUsersExcel(c)
	case "weekly":
		return exportWeeklyExcel(c)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid report type",
		})
	}
}

// exportUsersExcel exports user summaries to Excel
func exportUsersExcel(c *fiber.Ctx) error {
	summaries, err := database.GetUserSummaries()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load user data",
		})
	}

	// Create CSV content (simplified Excel export)
	csvContent := "Name,Email,Total Working Time (hours),Weekly Hours,Monthly Hours,Last Activity,Currently Working\n"
	
	for _, summary := range summaries {
		totalHours := float64(summary.TotalWorkingTime) / 3600.0
		workingStatus := "No"
		if summary.IsCurrentlyWorking {
			workingStatus = "Yes"
		}
		
		csvContent += fmt.Sprintf("%s,%s,%.2f,%.2f,%.2f,%s,%s\n",
			summary.Name,
			summary.Email,
			totalHours,
			summary.WeeklyHours,
			summary.MonthlyHours,
			summary.LastActivity.Format("2006-01-02 15:04:05"),
			workingStatus,
		)
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=users_report_%s.csv", time.Now().Format("2006-01-02")))
	
	return c.SendString(csvContent)
}

// exportWeeklyExcel exports weekly reports to Excel
func exportWeeklyExcel(c *fiber.Ctx) error {
	weekParam := c.Query("week")
	var weekStart time.Time
	var err error

	if weekParam != "" {
		weekStart, err = time.Parse("2006-01-02", weekParam)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid week format",
			})
		}
	} else {
		now := time.Now()
		weekStart = now.AddDate(0, 0, -int(now.Weekday())+1)
	}

	reports, err := database.GetWeeklyReports(weekStart)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load weekly reports",
		})
	}

	// Create CSV content
	csvContent := "Name,Email,Week Start,Week End,Total Hours,Required Hours,Completion Rate (%)\n"
	
	for _, report := range reports {
		csvContent += fmt.Sprintf("%s,%s,%s,%s,%.2f,%.2f,%.2f\n",
			report.Name,
			report.Email,
			report.WeekStart.Format("2006-01-02"),
			report.WeekEnd.Format("2006-01-02"),
			report.TotalHours,
			report.RequiredHours,
			report.CompletionRate,
		)
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=weekly_report_%s.csv", weekStart.Format("2006-01-02")))
	
	return c.SendString(csvContent)
}

// GetUserDetails returns detailed information about a specific user
func GetUserDetails(c *fiber.Ctx) error {
	userIDStr := c.Params("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var user database.User
	result := database.DB.Preload("TimeEntries").First(&user, uint(userID))
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(user)
}

// SyncSlackUsers manually syncs users from Slack
func SyncSlackUsers(c *fiber.Ctx) error {
	// This would typically be called by a Slack service
	// For now, return success
	return c.JSON(fiber.Map{
		"message": "User sync initiated",
	})
}

// HandleWebSocket handles WebSocket connections for real-time updates
func HandleWebSocket(c *websocket.Conn) {
	services.HandleWebSocket(c)
}

// calculateAnalytics calculates dashboard analytics from user summaries
func calculateAnalytics(summaries []database.UserSummary) map[string]interface{} {
	totalUsers := len(summaries)
	activeUsers := 0
	totalWeeklyHours := 0.0
	totalMonthlyHours := 0.0
	totalWorkingTime := int64(0)

	for _, summary := range summaries {
		if summary.IsCurrentlyWorking {
			activeUsers++
		}
		totalWeeklyHours += summary.WeeklyHours
		totalMonthlyHours += summary.MonthlyHours
		totalWorkingTime += summary.TotalWorkingTime
	}

	avgWeeklyHours := 0.0
	avgMonthlyHours := 0.0
	if totalUsers > 0 {
		avgWeeklyHours = totalWeeklyHours / float64(totalUsers)
		avgMonthlyHours = totalMonthlyHours / float64(totalUsers)
	}

	// Calculate completion rates (based on 20 hours per week requirement)
	weeklyTarget := 20.0 * float64(totalUsers)
	monthlyTarget := 80.0 * float64(totalUsers) // 4 weeks
	
	weeklyCompletion := 0.0
	monthlyCompletion := 0.0
	
	if weeklyTarget > 0 {
		weeklyCompletion = (totalWeeklyHours / weeklyTarget) * 100
	}
	if monthlyTarget > 0 {
		monthlyCompletion = (totalMonthlyHours / monthlyTarget) * 100
	}

	return map[string]interface{}{
		"total_users":         totalUsers,
		"active_users":        activeUsers,
		"total_weekly_hours":  totalWeeklyHours,
		"total_monthly_hours": totalMonthlyHours,
		"avg_weekly_hours":    avgWeeklyHours,
		"avg_monthly_hours":   avgMonthlyHours,
		"total_working_time":  float64(totalWorkingTime) / 3600.0, // Convert to hours
		"weekly_completion":   weeklyCompletion,
		"monthly_completion":  monthlyCompletion,
	}
} 

// GetLogStatsAPI returns logging statistics and configuration
func GetLogStatsAPI(c *fiber.Ctx) error {
	stats := utils.GetLogStats()
	
	// Add additional runtime information
	stats["verbose_logging_enabled"] = utils.IsVerboseEnabled()
	
	return c.JSON(fiber.Map{
		"log_statistics": stats,
		"status": "active",
	})
}

// RotateLogsAPI manually triggers log rotation
func RotateLogsAPI(c *fiber.Ctx) error {
	err := utils.RotateLogs()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to rotate logs",
			"details": err.Error(),
		})
	}

	// Get updated stats after rotation
	stats := utils.GetLogStats()
	
	return c.JSON(fiber.Map{
		"message": "Log rotation completed successfully",
		"new_stats": stats,
	})
} 