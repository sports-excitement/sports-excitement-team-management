package services

import (
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
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

				// Process the event
				go s.processSlackEvent(evt)

				// Acknowledge the event
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

// processSlackEvent processes incoming Slack events
func (s *SlackService) processSlackEvent(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		utils.LogVerbose("Event is not an EventsAPIEvent")
		return
	}

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.UserStatusChangedEvent:
			utils.LogVerbose("User status changed event: %+v", ev)
			s.handleUserStatusChanged(&ev.User)

			// Note: Commenting out UserChangeEvent to avoid duplicate processing
			// UserChangeEvent includes status changes which are already handled by UserStatusChangedEvent
			/*
				case *slackevents.UserChangeEvent:
					utils.LogVerbose("User change event: %+v", ev.User)
					s.handleUserChanged(&ev.User)
			*/
		}
	}
}

// handleUserStatusChanged processes user status change events
func (s *SlackService) handleUserStatusChanged(user *slackevents.User) {
	// Get user info to check current status and presence
	userInfo, err := s.client.GetUserInfo(user.ID)
	if err != nil {
		utils.LogError("Error getting user info for status change %s: %v", user.ID, err)
		return
	}

	if userInfo.Profile.Email == "" {
		utils.LogVerbose("No email found for user %s, skipping status change", user.ID)
		return
	}

	// Check if user is actually online/active in Slack
	// Handle cases where presence might be empty or unavailable
	isUserActive := userInfo.Presence == "active"
	if userInfo.Presence == "" {
		// If presence is unavailable, we'll process the status change but log it
		utils.LogVerbose("User %s presence unavailable, processing status change anyway", userInfo.Name)
		isUserActive = true // Assume active if we can't determine presence
	}

	if !isUserActive {
		utils.LogVerbose("User %s is not active in Slack (presence: %s), marking as offline", userInfo.Name, userInfo.Presence)

		// Find or create user in database for record keeping
		dbUser, err := database.CreateOrUpdateUser(
			user.ID,
			userInfo.Name,
			userInfo.Profile.Email,
			userInfo.RealName,
			userInfo.Profile.Image192,
		)
		if err != nil {
			utils.LogError("Error creating/updating user: %v", err)
			return
		}

		// End any active time entries since user is offline
		err = database.EndTimeEntry(dbUser.ID)
		if err != nil {
			utils.LogError("Error ending time entry for offline user: %v", err)
		}

		// Create status record for offline state
		s.processUserStatusChange(dbUser, "", "offline", false)
		return
	}

	// Find or create user in database
	dbUser, err := database.CreateOrUpdateUser(
		user.ID,
		userInfo.Name,
		userInfo.Profile.Email,
		userInfo.RealName,
		userInfo.Profile.Image192,
	)
	if err != nil {
		utils.LogError("Error creating/updating user during status change: %v", err)
		return
	}

	// Update last activity
	err = database.UpdateUserLastActivity(dbUser.ID)
	if err != nil {
		utils.LogError("Error updating user last activity: %v", err)
	}

	// Process status change with presence validation
	s.processUserStatusChange(dbUser, userInfo.Profile.StatusEmoji, userInfo.Profile.StatusText, true)
}

// handleUserChanged processes user change events (includes status changes)
func (s *SlackService) handleUserChanged(user *slackevents.User) {
	// Get fresh user info to access complete profile
	userInfo, err := s.client.GetUserInfo(user.ID)
	if err != nil {
		utils.LogError("Error getting user info for user change %s: %v", user.ID, err)
		return
	}

	if userInfo.Profile.Email == "" {
		utils.LogVerbose("No email found for user %s, skipping user change", user.ID)
		return
	}

	// Find or create user in database
	dbUser, err := database.CreateOrUpdateUser(
		user.ID,
		userInfo.Name,
		userInfo.Profile.Email,
		userInfo.RealName,
		userInfo.Profile.Image192,
	)
	if err != nil {
		utils.LogError("Error creating/updating user during user change: %v", err)
		return
	}

	// Update last activity
	err = database.UpdateUserLastActivity(dbUser.ID)
	if err != nil {
		utils.LogError("Error updating user last activity: %v", err)
	}

	// Process status change
	s.processUserStatusChange(dbUser, userInfo.Profile.StatusEmoji, userInfo.Profile.StatusText, true)
}

// processUserStatusChange handles the logic for status changes with deduplication
func (s *SlackService) processUserStatusChange(dbUser *database.User, statusEmoji, statusText string, isOnline bool) {
	// Check if this is actually a status change by comparing with latest status
	latestStatus, err := database.GetLatestUserStatus(dbUser.ID)
	if err == nil {
		// If same status as before, skip processing to avoid duplicates
		if latestStatus.StatusEmoji == statusEmoji && latestStatus.StatusText == statusText {
			utils.LogVerbose("User %s status unchanged (%s %s), skipping duplicate processing", dbUser.Name, statusEmoji, statusText)
			return
		}
	}

	var isWorking bool
	if !isOnline {
		// If user is offline, they're definitely not working
		isWorking = false
		statusText = "offline"
		statusEmoji = ""
	} else {
		// Only check working status if user is online
		isWorking = s.isWorkingStatus(statusEmoji, statusText)
		isNotWorking := s.isNotWorkingStatus(statusEmoji, statusText)

		// If explicitly not working, set to false
		if isNotWorking {
			isWorking = false
		}
	}

	// Store status change in database only if it's different from previous
	_, err = database.CreateUserStatus(dbUser.ID, statusEmoji, statusText, isWorking)
	if err != nil {
		utils.LogError("Error creating user status record: %v", err)
		return
	}

	if isWorking && isOnline {
		utils.LogInfo("User %s started working with status: %s %s", dbUser.Name, statusEmoji, statusText)

		_, err := database.StartTimeEntry(dbUser.ID, "Working", statusText, statusEmoji)
		if err != nil {
			utils.LogError("Error starting time entry: %v", err)
		}
	} else if !isWorking {
		if !isOnline {
			utils.LogInfo("User %s went offline", dbUser.Name)
		} else {
			utils.LogInfo("User %s stopped working with status: %s %s", dbUser.Name, statusEmoji, statusText)
		}

		err := database.EndTimeEntry(dbUser.ID)
		if err != nil {
			utils.LogError("Error ending time entry: %v", err)
		}
	} else {
		utils.LogVerbose("User %s has neutral status: %s %s - maintaining current state", dbUser.Name, statusEmoji, statusText)
	}

	// Broadcast user update
	if globalHub != nil {
		globalHub.BroadcastUserUpdate(dbUser.ID)
	}
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

// NOTE: CheckUserStatuses and checkUserStatus functions have been disabled
// to prevent conflicts with real-time event processing.
// All status changes are now handled via WebSocket events in real-time.

/*
func (s *SlackService) CheckUserStatuses() error {
	// DISABLED: This function conflicted with real-time event processing
	// causing duplicate status records and incorrect working states
	utils.LogVerbose("Periodic status checking disabled - using real-time events only")
	return nil
}

func (s *SlackService) checkUserStatus(userID string) {
	// DISABLED: This function has been replaced by real-time event handlers
	// that provide more accurate and immediate status tracking
	utils.LogVerbose("Manual status checking disabled - using real-time events only")
}
*/

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
		":gear:", ":bulb:", ":pencil:", ":memo:", ":working:",
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
		"vacation", "sick", "commuting",
		"traveling", "afk", "be right back", "brb",
	}

	notWorkingEmojis := []string{
		":lunch:", ":hamburger:", ":sandwich:", ":pizza:",
		":away:", ":zzz:", ":sleeping:", ":bed:",
		":car:", ":bus:", ":train:", ":airplane:",
		":face_with_thermometer:", ":sick:", ":sneezing_face:",
		":no_entry:", ":palm_tree:", ":spiral_calendar:",
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

	// NOTE: Removed periodic status checking to avoid conflicts with real-time events
	// Real-time events via WebSocket will handle all status changes
	utils.LogInfo("Real-time status tracking enabled via WebSocket events")

	// Start periodic duration updates only
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
