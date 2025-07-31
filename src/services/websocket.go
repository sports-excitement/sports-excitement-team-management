package services

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"

	"sports-excitement-team-management/src/database"
	"sports-excitement-team-management/src/utils"
)

// WebSocketHub manages WebSocket connections and broadcasting
type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.RWMutex
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Global hub instance for use by other services
var globalHub *WebSocketHub

// SetGlobalHub sets the global hub instance
func SetGlobalHub(hub *WebSocketHub) {
	globalHub = hub
}

// GetGlobalHub returns the global hub instance
func GetGlobalHub() *WebSocketHub {
	return globalHub
}

// Run starts the WebSocket hub
func (hub *WebSocketHub) Run() {
	for {
		select {
		case client := <-hub.register:
			hub.mutex.Lock()
			hub.clients[client] = true
			hub.mutex.Unlock()
			
			// Send initial data to the new client
			go hub.sendInitialData(client)

		case client := <-hub.unregister:
			hub.mutex.Lock()
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				client.Close()
			}
			hub.mutex.Unlock()

		case message := <-hub.broadcast:
			hub.mutex.RLock()
			for client := range hub.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					delete(hub.clients, client)
					client.Close()
				}
			}
			hub.mutex.RUnlock()
		}
	}
}

// sendInitialData sends initial dashboard data to a newly connected client
func (hub *WebSocketHub) sendInitialData(client *websocket.Conn) {
	// Get user summaries
	userSummaries, err := database.GetUserSummaries()
	if err != nil {
		utils.LogError("Error getting user summaries: %v", err)
		return
	}

	// Get analytics data
	analytics, err := database.GetAnalytics()
	if err != nil {
		utils.LogError("Error getting analytics: %v", err)
		return
	}

	initialData := map[string]interface{}{
		"type": "initial_data",
		"data": map[string]interface{}{
			"users":     userSummaries,
			"analytics": analytics,
		},
	}

	data, err := json.Marshal(initialData)
	if err != nil {
		utils.LogError("Error marshaling initial data: %v", err)
		return
	}

	// Use a timeout to avoid blocking
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		utils.LogVerbose("Initial data sent to WebSocket client")
	case <-time.After(5 * time.Second):
		utils.LogVerbose("Timeout sending initial data to client")
	}

	client.WriteMessage(websocket.TextMessage, data)
}

// BroadcastUserUpdate broadcasts a user update to all connected clients
func (hub *WebSocketHub) BroadcastUserUpdate(userID uint) {
	userSummary, err := database.GetUserSummary(userID)
	if err != nil {
		utils.LogError("Error getting user summary for broadcast: %v", err)
		return
	}

	updateData := map[string]interface{}{
		"type": "single_user_update",
		"data": map[string]interface{}{
			"user": userSummary,
		},
	}

	data, err := json.Marshal(updateData)
	if err != nil {
		utils.LogError("Error marshaling user update: %v", err)
		return
	}

	// Non-blocking send to broadcast channel
	select {
	case hub.broadcast <- data:
		// Message sent successfully
	default:
		utils.LogVerbose("Broadcast channel is full, skipping user update")
	}
}

// BroadcastMessage broadcasts a generic message to all connected clients
func (hub *WebSocketHub) BroadcastMessage(messageType string, data interface{}) {
	message := map[string]interface{}{
		"type": messageType,
		"data": data,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		utils.LogError("Error marshaling broadcast message: %v", err)
		return
	}

	// Non-blocking send to broadcast channel
	select {
	case hub.broadcast <- jsonData:
		// Message sent successfully
	default:
		utils.LogVerbose("Broadcast channel is full, skipping message")
	}
}

// GetClientCount returns the number of connected clients
func (hub *WebSocketHub) GetClientCount() int {
	hub.mutex.RLock()
	defer hub.mutex.RUnlock()
	return len(hub.clients)
}

// BroadcastAnalyticsUpdate broadcasts updated analytics to all clients
func (hub *WebSocketHub) BroadcastAnalyticsUpdate() {
	// Get fresh analytics data
	analytics, err := database.GetAnalytics()
	if err != nil {
		utils.LogError("Error getting analytics for broadcast: %v", err)
		return
	}

	// Get fresh user summaries too
	userSummaries, err := database.GetUserSummaries()
	if err != nil {
		utils.LogError("Error getting user summaries for analytics: %v", err)
		return
	}

	updateData := map[string]interface{}{
		"type": "user_update",
		"data": map[string]interface{}{
			"users":     userSummaries,
			"analytics": analytics,
		},
	}

	data, err := json.Marshal(updateData)
	if err != nil {
		utils.LogError("Error marshaling analytics update: %v", err)
		return
	}

	// Non-blocking send to broadcast channel
	select {
	case hub.broadcast <- data:
		// Message sent successfully
	default:
		utils.LogVerbose("Broadcast channel is full")
	}
}

// BroadcastToAll sends a message to all connected clients immediately
func (hub *WebSocketHub) BroadcastToAll(message []byte) {
	hub.mutex.RLock()
	defer hub.mutex.RUnlock()

	for client := range hub.clients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			utils.LogError("Error broadcasting to client: %v", err)
			// Remove disconnected client
			delete(hub.clients, client)
			client.Close()
		}
	}
}

// HandleWebSocket handles individual WebSocket connections
func HandleWebSocket(c *websocket.Conn) {
	if c == nil {
		utils.LogError("Received nil WebSocket connection")
		return
	}

	// Set global hub if not set
	if globalHub == nil {
		utils.LogError("Global WebSocket hub not initialized")
		return
	}

	// Register this connection with the hub
	globalHub.register <- c

	// Get current client count for logging
	clientCount := globalHub.GetClientCount()
	utils.LogInfo("WebSocket client connected directly. Total clients: %d", clientCount)

	// Set up ping/pong handlers for connection health
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start a goroutine to send periodic pings
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	// Handle disconnection
	defer func() {
		globalHub.unregister <- c
		c.Close()
		clientCount := globalHub.GetClientCount()
		utils.LogVerbose("WebSocket client disconnected. Total clients: %d", clientCount)
	}()

	// Read messages from the client (mainly for ping/pong)
	for {
		messageType, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				utils.LogError("WebSocket unexpected close error: %v", err)
			}
			break
		}

		// Handle ping messages
		if messageType == websocket.PingMessage {
			if err := c.WriteMessage(websocket.PongMessage, nil); err != nil {
				utils.LogError("WebSocket pong error: %v", err)
				break
			}
		}
	}
}

// Periodic task to broadcast analytics updates
func (hub *WebSocketHub) StartPeriodicUpdates() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if hub.GetClientCount() > 0 {
				hub.BroadcastAnalyticsUpdate()
			}
		}
	}()
}

// WriteMessage safely writes a message to a WebSocket connection
func (hub *WebSocketHub) WriteMessage(conn *websocket.Conn, messageType int, data []byte) error {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()

	if _, exists := hub.clients[conn]; !exists {
		return websocket.ErrCloseSent
	}

	err := conn.WriteMessage(messageType, data)
	if err != nil {
		utils.LogError("WebSocket write error: %v", err)
		delete(hub.clients, conn)
		conn.Close()
	}
	return err
}

// SendPing sends a ping message to a specific connection
func (hub *WebSocketHub) SendPing(conn *websocket.Conn) error {
	err := conn.WriteMessage(websocket.PingMessage, nil)
	if err != nil {
		utils.LogError("WebSocket ping error: %v", err)
		hub.mutex.Lock()
		delete(hub.clients, conn)
		conn.Close()
		hub.mutex.Unlock()
	}
	return err
} 
