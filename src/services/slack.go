package services

import (
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"

	"sports-excitement-team-management/src/config"
	"sports-excitement-team-management/src/database"
	"sports-excitement-team-management/src/utils"
)

type SlackService struct {
	client       *slack.Client
	socketClient *socketmode.Client
}

func NewSlackService() *SlackService {
	if config.AppConfig == nil {
		config.Init()
	}

	// Only enable Slack debug mode if verbose logging is enabled
	var options []slack.Option
	if config.AppConfig.EnableVerboseLogs {
		options = append(options, slack.OptionDebug(true))
	}
	options = append(options, slack.OptionAppLevelToken(config.AppConfig.SlackAppToken))

	client := slack.New(config.AppConfig.SlackBotToken, options...)
	socketClient := socketmode.New(client)

	return &SlackService{
		client:       client,
		socketClient: socketClient,
	}
}

func (s *SlackService) Start() {
	utils.LogInfo("Starting Slack Socket Mode connection...")

	go func() {
		for evt := range s.socketClient.Events {
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				utils.LogVerbose("Connecting to Slack with Socket Mode...")

			case socketmode.EventTypeConnectionError:
				utils.LogError("Connection failed. Retrying later...")

			case socketmode.EventTypeConnected:
				utils.LogInfo("Connected to Slack with Socket Mode.")

			case socketmode.EventTypeEventsAPI:
				utils.LogVerbose("Event received: %+v", evt)
				s.socketClient.Ack(*evt.Request)

			case socketmode.EventTypeInteractive:
				utils.LogVerbose("Interactive event received: %+v", evt)
				s.socketClient.Ack(*evt.Request)

			case socketmode.EventTypeSlashCommand:
				utils.LogVerbose("Slash command received: %+v", evt)
				s.socketClient.Ack(*evt.Request)
			}
		}
	}()

	s.socketClient.Run()
}

// SyncUsers synchronizes all users from Slack to the database
func (s *SlackService) SyncUsers() error {
	users, err := s.client.GetUsers()
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.IsBot || user.Deleted {
			continue
		}

		if user.Profile.Email == "" {
			continue
		}

		// Create or update user in database
		_, err := database.CreateOrUpdateUser(
			user.ID,
			user.Name,
			user.Profile.Email,
			user.RealName,
			user.Profile.Image192,
		)
		if err != nil {
			utils.LogError("Error syncing user %s: %v", user.Name, err)
			continue
		}
	}

	utils.LogVerbose("Synced %d users from Slack", len(users))
	return nil
}

func (s *SlackService) CheckUserStatuses() error {
	users, err := s.client.GetUsers()
	if err != nil {
		utils.LogError("Error getting user list for status check: %v", err)
		return err
	}

	for _, user := range users {
		if user.IsBot || user.Deleted {
			continue
		}
		s.checkUserStatus(user.ID)
	}

	return nil
}

func (s *SlackService) checkUserStatus(userID string) {
	utils.LogVerbose("Checking user status for user: %s", userID)

	// Get user info to access profile
	userInfo, err := s.client.GetUserInfo(userID)
	if err != nil {
		utils.LogError("Error getting user info for %s: %v", userID, err)
		return
	}

	if userInfo.Profile.Email == "" {
		utils.LogVerbose("No email found for user %s, skipping", userID)
		return
	}

	// Create or update user in database using existing function
	dbUser, err := database.CreateOrUpdateUser(
		userID,
		userInfo.Name,
		userInfo.Profile.Email,
		userInfo.RealName,
		userInfo.Profile.Image192,
	)
	if err != nil {
		utils.LogError("Error creating/updating user: %v", err)
		return
	}

	// Check if user is currently working based on status
	statusEmoji := userInfo.Profile.StatusEmoji
	status := userInfo.Profile.StatusText
	isWorking := s.isWorkingStatus(statusEmoji, status)

	if isWorking {
		utils.LogInfo("User %s started working with status: %s %s", userInfo.Name, statusEmoji, status)
		
		_, err := database.StartTimeEntry(dbUser.ID, "Working", status, statusEmoji)
		if err != nil {
			utils.LogError("Error starting time entry: %v", err)
		}
	} else if s.isNotWorkingStatus(statusEmoji, status) {
		utils.LogInfo("User %s stopped working with status: %s %s", userInfo.Name, statusEmoji, status)
		
		err := database.EndTimeEntry(dbUser.ID)
		if err != nil {
			utils.LogError("Error ending time entry: %v", err)
		}
	} else {
		utils.LogVerbose("User %s has neutral status: %s %s - maintaining current state", userInfo.Name, statusEmoji, status)
	}

	// Broadcast user update
	if globalHub != nil {
		globalHub.BroadcastUserUpdate(dbUser.ID)
	}
}

// isWorkingStatus checks if the status indicates the user is working
func (s *SlackService) isWorkingStatus(statusEmoji, statusText string) bool {
	// Convert to lowercase for easier matching
	statusText = strings.ToLower(statusText)
	statusEmoji = strings.ToLower(statusEmoji)

	workingKeywords := []string{
		"working", "coding", "developing", "programming", "building",
		"debugging", "testing", "reviewing", "meeting", "call",
		"designing", "planning", "writing", "documenting",
	}

	workingEmojis := []string{
		":computer:", ":laptop:", ":desktop_computer:", ":keyboard:",
		":coffee:", ":construction:", ":wrench:", ":hammer:",
		":gear:", ":bulb:", ":pencil:", ":memo:",
	}

	// Check status text
	for _, keyword := range workingKeywords {
		if strings.Contains(statusText, keyword) {
			return true
		}
	}

	// Check status emoji
	for _, emoji := range workingEmojis {
		if strings.Contains(statusEmoji, emoji) {
			return true
		}
	}

	return false
}

// isNotWorkingStatus checks if the status explicitly indicates the user is not working
func (s *SlackService) isNotWorkingStatus(statusEmoji, statusText string) bool {
	// Convert to lowercase for easier matching
	statusText = strings.ToLower(statusText)
	statusEmoji = strings.ToLower(statusEmoji)

	notWorkingKeywords := []string{
		"lunch", "break", "away", "out", "offline",
		"vacation", "sick", "meeting", "commuting",
		"traveling", "afk", "be right back", "brb",
	}

	notWorkingEmojis := []string{
		":lunch:", ":hamburger:", ":sandwich:", ":pizza:",
		":away:", ":zzz:", ":sleeping:", ":bed:",
		":car:", ":bus:", ":train:", ":airplane:",
		":face_with_thermometer:", ":sick:", ":sneezing_face:",
	}

	// Check status text
	for _, keyword := range notWorkingKeywords {
		if strings.Contains(statusText, keyword) {
			return true
		}
	}

	// Check status emoji
	for _, emoji := range notWorkingEmojis {
		if strings.Contains(statusEmoji, emoji) {
			return true
		}
	}

	return false
}

func (s *SlackService) StartWithInitialSync() {
	utils.LogInfo("Performing initial user sync...")
	if err := s.SyncUsers(); err != nil {
		utils.LogError("Error during initial user sync: %v", err)
	}

	utils.LogInfo("Checking initial user statuses...")
	if err := s.CheckUserStatuses(); err != nil {
		utils.LogError("Error checking initial user statuses: %v", err)
	}

	// Start the periodic status check
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			utils.LogVerbose("Performing periodic user status check...")
			if err := s.CheckUserStatuses(); err != nil {
				utils.LogError("Error during periodic status check: %v", err)
			}
		}
	}()

	// Start periodic duration updates
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.updateActiveDurations()
		}
	}()

	go s.Start()
}

func (s *SlackService) updateActiveDurations() {
	var activeEntries []database.TimeEntry
	err := database.DB.Where("end_time IS NULL").Find(&activeEntries).Error
	if err != nil {
		utils.LogError("Error fetching active time entries: %v", err)
		return
	}

	now := time.Now()
	for _, entry := range activeEntries {
		if entry.StartTime.IsZero() {
			continue
		}

		duration := int64(now.Sub(entry.StartTime).Seconds())
		
		// Update duration in database
		err := database.DB.Model(&entry).Update("duration", duration).Error
		if err != nil {
			utils.LogError("Error updating time entry duration for user %d: %v", entry.UserID, err)
			continue
		}

		// Broadcast update
		if globalHub != nil {
			globalHub.BroadcastUserUpdate(entry.UserID)
		}
	}

	utils.LogVerbose("Updated %d active time entries", len(activeEntries))
} 