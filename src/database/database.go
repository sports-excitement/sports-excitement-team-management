package database

import (
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sports-excitement-team-management/src/config"
	"sports-excitement-team-management/src/utils"
)

var DB *gorm.DB

// Initialize sets up the database connection and runs migrations
func Initialize() {
	config.Init()

	// Ensure database directory exists
	if config.AppConfig.DatabasePath != "" {
		dbDir := filepath.Dir(config.AppConfig.DatabasePath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			utils.LogFatal("Failed to create database directory %s: %v", dbDir, err)
		}
		utils.LogVerbose("Database directory ensured: %s", dbDir)
	}

	var err error
	// Set GORM log level based on verbose logging setting
	var logLevel logger.LogLevel
	if config.AppConfig.EnableVerboseLogs {
		logLevel = logger.Info
	} else {
		logLevel = logger.Error // Only show errors and fatal messages
	}

	DB, err = gorm.Open(sqlite.Open(config.AppConfig.DatabasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})

	if err != nil {
		utils.LogFatal("Failed to connect to database: %v", err)
	}

	// Run migrations
	err = DB.AutoMigrate(
		&User{},
		&TimeEntry{},
		&UserStatus{},
		&Admin{},
		&Session{},
	)

	if err != nil {
		utils.LogFatal("Failed to migrate database: %v", err)
	}

	// Create default admin user
	createDefaultAdmin()

	utils.LogInfo("Database initialized successfully at: %s", config.AppConfig.DatabasePath)
}

// createDefaultAdmin creates the default admin user if it doesn't exist
func createDefaultAdmin() {
	var admin Admin
	result := DB.Where("username = ?", config.AppConfig.AdminUser).First(&admin)

	if result.Error == gorm.ErrRecordNotFound {
		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(config.AppConfig.AdminPass), bcrypt.DefaultCost)
		if err != nil {
			utils.LogFatal("Failed to hash password: %v", err)
		}

		admin = Admin{
			Username: config.AppConfig.AdminUser,
			Password: string(hashedPassword),
		}

		if err := DB.Create(&admin).Error; err != nil {
			utils.LogFatal("Failed to create admin user: %v", err)
		}

		utils.LogInfo("Default admin user created: %s", config.AppConfig.AdminUser)
	} else {
		utils.LogVerbose("Admin user already exists")
	}
}

// UserSummaryRaw is used for scanning raw SQL results
type UserSummaryRaw struct {
	UserID             uint    `json:"user_id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	TotalWorkingTime   int64   `json:"total_working_time"`
	LastActivity       string  `json:"last_activity"`        // String for SQLite datetime
	IsCurrentlyWorking int     `json:"is_currently_working"` // SQLite returns int for boolean
	CurrentStatus      string  `json:"current_status"`
	WeeklyHours        float64 `json:"weekly_hours"`
	MonthlyHours       float64 `json:"monthly_hours"`
}

// GetUserSummaries returns aggregated user data for dashboard
func GetUserSummaries() ([]UserSummary, error) {
	var rawSummaries []UserSummaryRaw

	query := `
		SELECT 
			u.id as user_id,
			COALESCE(NULLIF(u.real_name, ''), u.name) as name,
			u.email,
			COALESCE(SUM(te.duration), 0) as total_working_time,
			COALESCE(MAX(u.updated_at), u.created_at) as last_activity,
			CASE WHEN EXISTS(
				SELECT 1 FROM user_statuses us
				WHERE us.user_id = u.id 
				AND us.is_working = 1
				AND us.id = (
					SELECT MAX(us2.id) FROM user_statuses us2 
					WHERE us2.user_id = u.id
				)
			) THEN 1 ELSE 0 END as is_currently_working,
			COALESCE(us_current.status_text, '') as current_status,
			COALESCE(SUM(CASE 
				WHEN te.start_time >= date('now', '-7 days') 
				THEN te.duration ELSE 0 
			END), 0) / 3600.0 as weekly_hours,
			COALESCE(SUM(CASE 
				WHEN te.start_time >= date('now', 'start of month') 
				THEN te.duration ELSE 0 
			END), 0) / 3600.0 as monthly_hours
		FROM users u
		LEFT JOIN time_entries te ON u.id = te.user_id
		LEFT JOIN (
			SELECT DISTINCT user_id, status_text,
			ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY timestamp DESC) as rn
			FROM user_statuses
		) us_current ON u.id = us_current.user_id AND us_current.rn = 1
		WHERE u.is_active = 1
		GROUP BY u.id, u.name, u.email, us_current.status_text
		ORDER BY u.name
	`

	err := DB.Raw(query).Scan(&rawSummaries).Error
	if err != nil {
		return nil, err
	}

	// Convert raw results to proper UserSummary structs
	var summaries []UserSummary
	for _, raw := range rawSummaries {
		// Parse the datetime string
		lastActivity, err := time.Parse("2006-01-02 15:04:05", raw.LastActivity)
		if err != nil {
			// Try alternative format
			lastActivity, err = time.Parse(time.RFC3339, raw.LastActivity)
			if err != nil {
				// Fallback to current time if parsing fails
				lastActivity = time.Now()
			}
		}

		summary := UserSummary{
			UserID:             raw.UserID,
			Name:               raw.Name,
			Email:              raw.Email,
			TotalWorkingTime:   raw.TotalWorkingTime,
			LastActivity:       lastActivity,
			IsCurrentlyWorking: raw.IsCurrentlyWorking == 1,
			CurrentStatus:      raw.CurrentStatus,
			WeeklyHours:        raw.WeeklyHours,
			MonthlyHours:       raw.MonthlyHours,
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// WeeklyReportRaw is used for scanning raw SQL results
type WeeklyReportRaw struct {
	UserID         uint    `json:"user_id"`
	Name           string  `json:"name"`
	Email          string  `json:"email"`
	WeekStart      string  `json:"week_start"` // String for SQLite datetime
	WeekEnd        string  `json:"week_end"`   // String for SQLite datetime
	TotalHours     float64 `json:"total_hours"`
	RequiredHours  float64 `json:"required_hours"`
	CompletionRate float64 `json:"completion_rate"`
}

// GetWeeklyReports returns weekly time tracking reports
func GetWeeklyReports(weekStart time.Time) ([]WeeklyReport, error) {
	var rawReports []WeeklyReportRaw
	weekEnd := weekStart.AddDate(0, 0, 6)

	query := `
		SELECT 
			u.id as user_id,
			COALESCE(NULLIF(u.real_name, ''), u.name) as name,
			u.email,
			? as week_start,
			? as week_end,
			COALESCE(SUM(te.duration), 0) / 3600.0 as total_hours,
			20.0 as required_hours,
			(COALESCE(SUM(te.duration), 0) / 3600.0) / 20.0 * 100 as completion_rate
		FROM users u
		LEFT JOIN time_entries te ON u.id = te.user_id 
			AND te.start_time >= ? 
			AND te.start_time <= ?
		WHERE u.is_active = 1
		GROUP BY u.id, u.name, u.email
		ORDER BY u.name
	`

	err := DB.Raw(query, weekStart, weekEnd, weekStart, weekEnd).Scan(&rawReports).Error
	if err != nil {
		return nil, err
	}

	// Convert raw results to proper WeeklyReport structs
	var reports []WeeklyReport
	for _, raw := range rawReports {
		// Parse the datetime strings
		weekStartParsed, err := time.Parse("2006-01-02 15:04:05", raw.WeekStart)
		if err != nil {
			weekStartParsed, err = time.Parse(time.RFC3339, raw.WeekStart)
			if err != nil {
				weekStartParsed = weekStart // Fallback to provided parameter
			}
		}

		weekEndParsed, err := time.Parse("2006-01-02 15:04:05", raw.WeekEnd)
		if err != nil {
			weekEndParsed, err = time.Parse(time.RFC3339, raw.WeekEnd)
			if err != nil {
				weekEndParsed = weekEnd // Fallback to calculated end
			}
		}

		report := WeeklyReport{
			UserID:         raw.UserID,
			Name:           raw.Name,
			Email:          raw.Email,
			WeekStart:      weekStartParsed,
			WeekEnd:        weekEndParsed,
			TotalHours:     raw.TotalHours,
			RequiredHours:  raw.RequiredHours,
			CompletionRate: raw.CompletionRate,
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// CreateOrUpdateUser creates or updates a user from Slack data
func CreateOrUpdateUser(slackUserID, name, email, realName, profileImage string) (*User, error) {
	var user User

	result := DB.Where("slack_user_id = ?", slackUserID).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new user
		user = User{
			SlackUserID:  slackUserID,
			Name:         name,
			Email:        email,
			RealName:     realName,
			ProfileImage: profileImage,
			IsActive:     true,
		}
		err := DB.Create(&user).Error
		return &user, err
	} else if result.Error != nil {
		return nil, result.Error
	}

	// Update existing user
	user.Name = name
	user.Email = email
	user.RealName = realName
	user.ProfileImage = profileImage

	err := DB.Save(&user).Error
	return &user, err
}

// StartTimeEntry starts a new time tracking entry
func StartTimeEntry(userID uint, status, statusText, statusEmoji string) (*TimeEntry, error) {
	// End any existing active entries for this user
	DB.Model(&TimeEntry{}).
		Where("user_id = ? AND end_time IS NULL", userID).
		Update("end_time", time.Now())

	// Create new entry
	entry := TimeEntry{
		UserID:      userID,
		StartTime:   time.Now(),
		Status:      status,
		StatusText:  statusText,
		StatusEmoji: statusEmoji,
	}

	err := DB.Create(&entry).Error
	return &entry, err
}

// EndTimeEntry ends an active time tracking entry
func EndTimeEntry(userID uint) error {
	now := time.Now()

	var entry TimeEntry
	result := DB.Where("user_id = ? AND end_time IS NULL", userID).First(&entry)

	if result.Error == gorm.ErrRecordNotFound {
		return nil // No active entry to end
	} else if result.Error != nil {
		return result.Error
	}

	// Calculate duration and update entry
	duration := now.Sub(entry.StartTime).Seconds()
	entry.EndTime = &now
	entry.Duration = int64(duration)

	return DB.Save(&entry).Error
}

// CreateUserStatus creates a new user status record
func CreateUserStatus(userID uint, statusEmoji, statusText string, isWorking bool) (*UserStatus, error) {
	status := UserStatus{
		UserID:      userID,
		StatusEmoji: statusEmoji,
		StatusText:  statusText,
		IsWorking:   isWorking,
		Timestamp:   time.Now(),
	}

	err := DB.Create(&status).Error
	if err != nil {
		return nil, err
	}

	return &status, nil
}

// GetLatestUserStatus returns the most recent status for a user
func GetLatestUserStatus(userID uint) (*UserStatus, error) {
	var status UserStatus
	err := DB.Where("user_id = ?", userID).Order("timestamp DESC").First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// GetUserCurrentWorkingStatus checks if user is currently working based on latest status
func GetUserCurrentWorkingStatus(userID uint) (bool, error) {
	latestStatus, err := GetLatestUserStatus(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil // No status recorded, assume not working
		}
		return false, err
	}
	return latestStatus.IsWorking, nil
}

// UpdateUserLastActivity updates user's last activity timestamp
func UpdateUserLastActivity(userID uint) error {
	return DB.Model(&User{}).Where("id = ?", userID).Update("updated_at", time.Now()).Error
}

// CleanupDuplicateStatuses removes duplicate status entries keeping only the latest for each user
func CleanupDuplicateStatuses() error {
	utils.LogInfo("Starting cleanup of duplicate user status entries...")

	// Get all users
	var users []User
	err := DB.Find(&users).Error
	if err != nil {
		return err
	}

	duplicatesRemoved := 0

	for _, user := range users {
		// Get all statuses for this user, ordered by timestamp DESC
		var statuses []UserStatus
		err := DB.Where("user_id = ?", user.ID).Order("timestamp DESC").Find(&statuses).Error
		if err != nil {
			continue
		}

		// Group by same status (emoji + text) and keep only the latest
		seen := make(map[string]bool)
		var toDelete []uint

		for _, status := range statuses {
			statusKey := status.StatusEmoji + "|" + status.StatusText
			if seen[statusKey] {
				// This is a duplicate, mark for deletion
				toDelete = append(toDelete, status.ID)
				duplicatesRemoved++
			} else {
				seen[statusKey] = true
			}
		}

		// Delete duplicates
		if len(toDelete) > 0 {
			err := DB.Where("id IN ?", toDelete).Delete(&UserStatus{}).Error
			if err != nil {
				utils.LogError("Error deleting duplicate statuses for user %s: %v", user.Name, err)
			} else {
				utils.LogInfo("Removed %d duplicate status entries for user %s", len(toDelete), user.Name)
			}
		}
	}

	utils.LogInfo("Cleanup completed: removed %d duplicate status entries", duplicatesRemoved)
	return nil
}

// GetUserSummary returns a single user summary by ID
func GetUserSummary(userID uint) (UserSummary, error) {
	var rawSummary UserSummaryRaw

	query := `
		SELECT 
			u.id as user_id,
			COALESCE(NULLIF(u.real_name, ''), u.name) as name,
			u.email,
			COALESCE(SUM(te.duration), 0) as total_working_time,
			COALESCE(MAX(te.updated_at), u.created_at) as last_activity,
			CASE WHEN EXISTS(
				SELECT 1 FROM time_entries te2 
				WHERE te2.user_id = u.id 
				AND te2.end_time IS NULL 
				AND te2.status = 'Working'
			) THEN 1 ELSE 0 END as is_currently_working,
			COALESCE(te_current.status, '') as current_status,
			COALESCE(SUM(CASE 
				WHEN te.start_time >= date('now', '-7 days') 
				THEN te.duration ELSE 0 
			END), 0) / 3600.0 as weekly_hours,
			COALESCE(SUM(CASE 
				WHEN te.start_time >= date('now', 'start of month') 
				THEN te.duration ELSE 0 
			END), 0) / 3600.0 as monthly_hours
		FROM users u
		LEFT JOIN time_entries te ON u.id = te.user_id
		LEFT JOIN (
			SELECT DISTINCT user_id, status,
			ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at DESC) as rn
			FROM time_entries
		) te_current ON u.id = te_current.user_id AND te_current.rn = 1
		WHERE u.is_active = 1 AND u.id = ?
		GROUP BY u.id, u.name, u.email, te_current.status
	`

	err := DB.Raw(query, userID).Scan(&rawSummary).Error
	if err != nil {
		return UserSummary{}, err
	}

	// Parse the datetime string
	lastActivity, err := time.Parse("2006-01-02 15:04:05", rawSummary.LastActivity)
	if err != nil {
		lastActivity, err = time.Parse(time.RFC3339, rawSummary.LastActivity)
		if err != nil {
			lastActivity = time.Now()
		}
	}

	summary := UserSummary{
		UserID:             rawSummary.UserID,
		Name:               rawSummary.Name,
		Email:              rawSummary.Email,
		TotalWorkingTime:   rawSummary.TotalWorkingTime,
		LastActivity:       lastActivity,
		IsCurrentlyWorking: rawSummary.IsCurrentlyWorking == 1,
		CurrentStatus:      rawSummary.CurrentStatus,
		WeeklyHours:        rawSummary.WeeklyHours,
		MonthlyHours:       rawSummary.MonthlyHours,
	}

	return summary, nil
}

// Analytics represents analytics data for the dashboard
type Analytics struct {
	TotalUsers        int     `json:"total_users"`
	ActiveUsers       int     `json:"active_users"`
	TotalWeeklyHours  float64 `json:"total_weekly_hours"`
	TotalMonthlyHours float64 `json:"total_monthly_hours"`
	AvgWeeklyHours    float64 `json:"avg_weekly_hours"`
	AvgMonthlyHours   float64 `json:"avg_monthly_hours"`
}

// GetAnalytics returns analytics data for the dashboard
func GetAnalytics() (Analytics, error) {
	summaries, err := GetUserSummaries()
	if err != nil {
		return Analytics{}, err
	}

	totalUsers := len(summaries)
	activeUsers := 0
	totalWeeklyHours := 0.0
	totalMonthlyHours := 0.0

	for _, summary := range summaries {
		if summary.IsCurrentlyWorking {
			activeUsers++
		}
		totalWeeklyHours += summary.WeeklyHours
		totalMonthlyHours += summary.MonthlyHours
	}

	avgWeeklyHours := 0.0
	avgMonthlyHours := 0.0
	if totalUsers > 0 {
		avgWeeklyHours = totalWeeklyHours / float64(totalUsers)
		avgMonthlyHours = totalMonthlyHours / float64(totalUsers)
	}

	analytics := Analytics{
		TotalUsers:        totalUsers,
		ActiveUsers:       activeUsers,
		TotalWeeklyHours:  totalWeeklyHours,
		TotalMonthlyHours: totalMonthlyHours,
		AvgWeeklyHours:    avgWeeklyHours,
		AvgMonthlyHours:   avgMonthlyHours,
	}

	return analytics, nil
}
