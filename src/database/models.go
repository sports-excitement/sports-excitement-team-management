package database

import (
	"time"
)

// User represents a Slack user being tracked
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SlackUserID  string    `json:"slack_user_id" gorm:"uniqueIndex;not null"`
	Name         string    `json:"name" gorm:"not null"`
	Email        string    `json:"email" gorm:"not null"`
	RealName     string    `json:"real_name"`
	ProfileImage string    `json:"profile_image"`
	IsActive     bool      `json:"is_active" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relationships
	TimeEntries []TimeEntry `json:"time_entries" gorm:"foreignKey:UserID"`
}

// TimeEntry represents a time tracking entry for a user
type TimeEntry struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"not null"`
	StartTime   time.Time  `json:"start_time" gorm:"not null"`
	EndTime     *time.Time `json:"end_time"`
	Duration    int64      `json:"duration"` // Duration in seconds
	Status      string     `json:"status" gorm:"not null"`
	StatusText  string     `json:"status_text"`
	StatusEmoji string     `json:"status_emoji"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Relationships
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// UserSummary represents aggregated user data for dashboard
type UserSummary struct {
	UserID             uint      `json:"user_id"`
	Name               string    `json:"name"`
	Email              string    `json:"email"`
	TotalWorkingTime   int64     `json:"total_working_time"` // Total seconds worked
	LastActivity       time.Time `json:"last_activity"`
	IsCurrentlyWorking bool      `json:"is_currently_working"`
	CurrentStatus      string    `json:"current_status"`
	WeeklyHours        float64   `json:"weekly_hours"`
	MonthlyHours       float64   `json:"monthly_hours"`
}

// WeeklyReport represents weekly time tracking report
type WeeklyReport struct {
	UserID         uint      `json:"user_id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	WeekStart      time.Time `json:"week_start"`
	WeekEnd        time.Time `json:"week_end"`
	TotalHours     float64   `json:"total_hours"`
	RequiredHours  float64   `json:"required_hours"`
	CompletionRate float64   `json:"completion_rate"`
}

// Admin represents admin user session
type Admin struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"uniqueIndex;not null"`
	Password  string    `json:"password" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserStatus represents a historical record of user status changes from Slack
type UserStatus struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      uint      `json:"user_id" gorm:"not null"`
	StatusEmoji string    `json:"status_emoji"`
	StatusText  string    `json:"status_text"`
	IsWorking   bool      `json:"is_working" gorm:"default:false"`
	Timestamp   time.Time `json:"timestamp" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relationships
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// Session represents user session
type Session struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id"`
	Data      string    `json:"data"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
