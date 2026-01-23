package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/usecase"
	"tachyon-messenger/shared/redis"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gorilla/websocket"
)

// Upgrader with proper configuration
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin in development
		// In production, implement proper origin checking
		return true
	},
}

// HubMetrics contains hub statistics
type HubMetrics struct {
	ConnectedClients int       `json:"connected_clients"`
	ActiveChatRooms  int       `json:"active_chat_rooms"`
	MessagesSent     int64     `json:"messages_sent"`
	MessagesReceived int64     `json:"messages_received"`
	Uptime           time.Time `json:"uptime"`
}

// TypingIndicator represents a typing status
type TypingIndicator struct {
	UserID    uint      `json:"user_id"`
	ChatID    uint      `json:"chat_id"`
	IsTyping  bool      `json:"is_typing"`
	Timestamp time.Time `json:"timestamp"`
}

// UserPresence represents user online status
type UserPresence struct {
	UserID       uint      `json:"user_id"`
	Status       string    `json:"status"` // online, away, busy, offline
	LastSeen     time.Time `json:"last_seen"`
	LastActiveAt time.Time `json:"last_active_at"` // Alias for last_seen for frontend compatibility
	ChatRooms    []uint    `json:"chat_rooms,omitempty"`
}

// NewHub creates a new WebSocket hub
func NewHub(messageUsecase usecase.MessageUsecase, redisClient *redis.Client) *Hub {
	return &Hub{
		clients:        make(map[uint]map[string]*Client), // userID -> connectionID -> Client
		chatRooms:      make(map[uint]map[uint]bool),
		broadcast:      make(chan *BroadcastMessage, 1024),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		shutdown:       make(chan struct{}),
		messageUsecase: messageUsecase,
		chatRepo:       nil, // Will be set later via SetChatRepository
		redisClient:    redisClient,
		metrics: &HubMetrics{
			Uptime: time.Now(),
		},
	}
}

// SetChatRepository sets the chat repository for the hub
func (h *Hub) SetChatRepository(chatRepo ChatRepository) {
	h.chatRepo = chatRepo
	log.Println("✅ Chat repository set in WebSocket Hub")
}

// Run starts the hub and handles client connections
func (h *Hub) Run() {
	log.Println("WebSocket hub started")

	// Reset all stuck online statuses on startup
	go h.resetStuckOnlineStatuses()

	// Start metrics updater
	go h.updateMetrics()

	// Start periodic status cleanup (every 5 minutes)
	go h.periodicStatusCleanup()

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case <-h.shutdown:
			log.Println("WebSocket hub shutting down...")
			h.cleanup()
			return
		}
	}
}

// Close shuts down the hub gracefully
func (h *Hub) Close() {
	close(h.shutdown)
}

// registerClient registers a new client (supports multiple connections per user)
func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	userID := client.userID
	connID := client.connectionID

	// Initialize user's connections map if not exists
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[string]*Client)
	}

	// Check if this is the first connection for this user
	isFirstConnection := len(h.clients[userID]) == 0

	// Add the new connection
	h.clients[userID][connID] = client
	client.status = "online"
	client.lastSeen = time.Now()

	totalConnections := h.getTotalConnectionsCount()
	log.Printf("Client registered: user %d, connection %s (user has %d connections, total connections: %d)",
		userID, connID[:8], len(h.clients[userID]), totalConnections)

	// Only broadcast "online" status if this is the first connection for this user
	if isFirstConnection {
		h.broadcastUserPresence(userID, "online")
	}
}

// getTotalConnectionsCount returns total number of active connections across all users
func (h *Hub) getTotalConnectionsCount() int {
	count := 0
	for _, connections := range h.clients {
		count += len(connections)
	}
	return count
}

// unregisterClient unregisters a client (supports multiple connections per user)
func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	userID := client.userID
	connID := client.connectionID

	// Check if user has connections and this specific connection exists
	userConnections, userExists := h.clients[userID]
	if !userExists {
		return
	}

	storedClient, connExists := userConnections[connID]
	if !connExists || storedClient != client {
		return
	}

	// Close the send channel and remove this specific connection
	close(client.send)
	delete(userConnections, connID)

	// Check if this was the last connection for this user
	isLastConnection := len(userConnections) == 0

	if isLastConnection {
		// Remove user from clients map entirely
		delete(h.clients, userID)

		// Update status in user-service synchronously
		updateUserStatus(userID, "offline")

		// Broadcast offline presence
		h.broadcastOfflinePresence(client, userID)

		// Remove user from all in-memory chat rooms
		for chatID := range client.chatRooms {
			if users, exists := h.chatRooms[chatID]; exists {
				delete(users, userID)
				if len(users) == 0 {
					delete(h.chatRooms, chatID)
				}
			}
		}
	}

	totalConnections := h.getTotalConnectionsCount()
	log.Printf("Client unregistered: user %d, connection %s (user has %d remaining connections, total: %d, was last: %v)",
		userID, connID[:8], len(userConnections), totalConnections, isLastConnection)
}

// broadcastOfflinePresence broadcasts offline presence to relevant users
func (h *Hub) broadcastOfflinePresence(client *Client, userID uint) {
	now := time.Now()
	presence := &UserPresence{
		UserID:       userID,
		Status:       "offline",
		LastSeen:     now,
		LastActiveAt: now,
	}

	// Get user's chat IDs from database
	var chatRoomIDs []uint
	if h.chatRepo != nil {
		dbChatIDs, err := h.chatRepo.GetUserChatIDs(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get user chat IDs from DB: %v, falling back to in-memory", err)
			for chatID := range client.chatRooms {
				chatRoomIDs = append(chatRoomIDs, chatID)
			}
		} else {
			chatRoomIDs = dbChatIDs
		}
	} else {
		for chatID := range client.chatRooms {
			chatRoomIDs = append(chatRoomIDs, chatID)
		}
	}
	presence.ChatRooms = chatRoomIDs

	log.Printf("📢 Broadcasting offline status for user %d to %d chat rooms", userID, len(chatRoomIDs))

	// Collect all unique users across all chat rooms
	uniqueUsers := make(map[uint]bool)
	for _, chatID := range chatRoomIDs {
		var memberIDs []uint
		if h.chatRepo != nil {
			dbMemberIDs, err := h.chatRepo.GetChatMemberIDs(chatID)
			if err != nil {
				log.Printf("⚠️ Failed to get chat %d members from DB: %v", chatID, err)
				if users, exists := h.chatRooms[chatID]; exists {
					for otherUserID := range users {
						memberIDs = append(memberIDs, otherUserID)
					}
				}
			} else {
				memberIDs = dbMemberIDs
			}
		} else {
			if users, exists := h.chatRooms[chatID]; exists {
				for otherUserID := range users {
					memberIDs = append(memberIDs, otherUserID)
				}
			}
		}

		for _, otherUserID := range memberIDs {
			if otherUserID != userID {
				uniqueUsers[otherUserID] = true
			}
		}
	}

	// Create the message
	message := &BroadcastMessage{
		Type:      models.WSMessageType("user_presence"),
		ChatID:    0,
		UserID:    userID,
		Data:      presence,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling offline presence message: %v", err)
		return
	}

	// Send to each unique user's ALL connections
	sent := 0
	for otherUserID := range uniqueUsers {
		if otherConnections, exists := h.clients[otherUserID]; exists {
			for _, otherClient := range otherConnections {
				select {
				case otherClient.send <- messageBytes:
					sent++
				default:
					log.Printf("⚠️ Client %d send channel full, skipping offline presence", otherUserID)
				}
			}
		}
	}

	log.Printf("✅ Sent offline presence for user %d to %d connections (was in %d chats, %d total members)",
		userID, sent, len(chatRoomIDs), len(uniqueUsers))
}

// forceDisconnectClient forcefully disconnects a client
func (h *Hub) forceDisconnectClient(client *Client) {
	close(client.send)
	client.conn.Close()

	// Remove from chat rooms
	for chatID := range client.chatRooms {
		if users, exists := h.chatRooms[chatID]; exists {
			delete(users, client.userID)
			if len(users) == 0 {
				delete(h.chatRooms, chatID)
			}
		}
	}
}

// broadcastMessage broadcasts a message to relevant clients (all connections per user)
func (h *Hub) broadcastMessage(broadcastMsg *BroadcastMessage) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	broadcastMsg.Timestamp = time.Now()
	message, err := json.Marshal(broadcastMsg)
	if err != nil {
		log.Printf("Error marshaling broadcast message: %v", err)
		return
	}

	sent := 0

	// Get users who should receive this message based on chatID
	targetUserIDs := h.getChatMemberIDs(broadcastMsg.ChatID)

	for _, userID := range targetUserIDs {
		// Skip excluded user (e.g., message sender)
		if broadcastMsg.ExcludeUser != 0 && userID == broadcastMsg.ExcludeUser {
			continue
		}

		// Send to ALL user's connections (multi-device support)
		if userConnections, userExists := h.clients[userID]; userExists {
			for connID, client := range userConnections {
				select {
				case client.send <- message:
					sent++
				default:
					// Client's send channel is full, mark for removal
					log.Printf("Client %d connection %s send channel full", userID, connID[:8])
				}
			}
		}
	}

	h.metrics.MessagesSent += int64(sent)

	if sent > 0 {
		log.Printf("Broadcasted %s message to %d connections (chat %d)", broadcastMsg.Type, sent, broadcastMsg.ChatID)
	}
}

// getChatMemberIDs retrieves all member user IDs for a given chat
// NEW: Uses database repository for accurate membership data
func (h *Hub) getChatMemberIDs(chatID uint) []uint {
	if chatID == 0 {
		// For global broadcasts (like user_presence), return all connected users
		h.mutex.RLock()
		defer h.mutex.RUnlock()

		userIDs := make([]uint, 0, len(h.clients))
		for userID := range h.clients {
			userIDs = append(userIDs, userID)
		}
		return userIDs
	}

	// NEW: Get chat members from database via chatRepo
	if h.chatRepo != nil {
		userIDs, err := h.chatRepo.GetChatMemberIDs(chatID)
		if err != nil {
			log.Printf("⚠️ Failed to get chat member IDs from DB for chat %d: %v", chatID, err)
			// Fall back to chatRooms cache as backup
			if users, exists := h.chatRooms[chatID]; exists {
				fallbackIDs := make([]uint, 0, len(users))
				for userID := range users {
					fallbackIDs = append(fallbackIDs, userID)
				}
				return fallbackIDs
			}
			return []uint{}
		}
		return userIDs
	}

	// Fallback: Use in-memory chatRooms if repository not available
	if users, exists := h.chatRooms[chatID]; exists {
		userIDs := make([]uint, 0, len(users))
		for userID := range users {
			userIDs = append(userIDs, userID)
		}
		return userIDs
	}

	return []uint{}
}

// broadcastUserPresence broadcasts user presence change
func (h *Hub) broadcastUserPresence(userID uint, status string) {
	now := time.Now()
	presence := &UserPresence{
		UserID:       userID,
		Status:       status,
		LastSeen:     now,
		LastActiveAt: now,
	}

	// Update status in user-service
	go updateUserStatus(userID, status)

	// Get user's chat IDs from database (not just from in-memory chatRooms)
	// This ensures we notify all chat members even if user didn't explicitly join rooms
	var chatRoomIDs []uint
	if h.chatRepo != nil {
		dbChatIDs, err := h.chatRepo.GetUserChatIDs(userID)
		if err != nil {
			log.Printf("⚠️ Failed to get user chat IDs from DB: %v, falling back to in-memory", err)
			// Fallback to in-memory chatRooms
			for chatID, users := range h.chatRooms {
				if _, exists := users[userID]; exists {
					chatRoomIDs = append(chatRoomIDs, chatID)
				}
			}
		} else {
			chatRoomIDs = dbChatIDs
		}
	} else {
		// No chatRepo available, use in-memory chatRooms
		for chatID, users := range h.chatRooms {
			if _, exists := users[userID]; exists {
				chatRoomIDs = append(chatRoomIDs, chatID)
			}
		}
	}
	presence.ChatRooms = chatRoomIDs

	log.Printf("Broadcasting user_presence for user %d (status: %s) to %d chats", userID, status, len(chatRoomIDs))

	// Collect all unique users across all chat rooms (except the user themselves)
	// Use database to get members for each chat (more reliable than in-memory)
	uniqueUsers := make(map[uint]bool)
	for _, chatID := range chatRoomIDs {
		var memberIDs []uint
		if h.chatRepo != nil {
			dbMemberIDs, err := h.chatRepo.GetChatMemberIDs(chatID)
			if err != nil {
				log.Printf("⚠️ Failed to get chat %d members from DB: %v", chatID, err)
				// Fallback to in-memory
				if users, exists := h.chatRooms[chatID]; exists {
					for otherUserID := range users {
						memberIDs = append(memberIDs, otherUserID)
					}
				}
			} else {
				memberIDs = dbMemberIDs
			}
		} else {
			// Fallback to in-memory chatRooms
			if users, exists := h.chatRooms[chatID]; exists {
				for otherUserID := range users {
					memberIDs = append(memberIDs, otherUserID)
				}
			}
		}

		for _, otherUserID := range memberIDs {
			if otherUserID != userID {
				uniqueUsers[otherUserID] = true
			}
		}
	}

	// Create the message once
	message := &BroadcastMessage{
		Type:      models.WSMessageType("user_presence"),
		ChatID:    0, // Not specific to one chat
		UserID:    userID,
		Data:      presence,
		Timestamp: time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling presence message: %v", err)
		return
	}

	// Send to each unique user's ALL connections (multi-device support)
	sent := 0
	for otherUserID := range uniqueUsers {
		if userConnections, exists := h.clients[otherUserID]; exists {
			for _, client := range userConnections {
				select {
				case client.send <- messageBytes:
					sent++
				default:
					log.Printf("Client %d send channel full, skipping presence update", otherUserID)
				}
			}
		}
	}

	log.Printf("✅ Sent user_presence for user %d to %d connections (was in %d chats, %d total members)",
		userID, sent, len(chatRoomIDs), len(uniqueUsers))
}

// updateUserStatus updates user status in user-service via HTTP
func updateUserStatus(userID uint, status string) {
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8081"
	}

	// Map status string to UserStatus type
	var userStatus sharedmodels.UserStatus
	switch status {
	case "online":
		userStatus = sharedmodels.StatusOnline
	case "away":
		userStatus = sharedmodels.StatusAway
	case "busy":
		userStatus = sharedmodels.StatusBusy
	case "offline":
		userStatus = sharedmodels.StatusOffline
	default:
		userStatus = sharedmodels.StatusOffline
	}

	payload := map[string]interface{}{
		"status": userStatus,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Failed to marshal status update for user %d: %v", userID, err)
		return
	}

	url := fmt.Sprintf("%s/internal/users/%d/status", userServiceURL, userID)

	// Create request with context for better timeout control
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create status update request for user %d: %v", userID, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to update status for user %d in user-service: %v", userID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ User service returned non-OK status %d for user %d status update", resp.StatusCode, userID)
		return
	}

	log.Printf("✅ Updated user %d status to %s in user-service", userID, status)
}

// JoinChatRoom adds a user to a chat room (updates all user connections)
func (h *Hub) JoinChatRoom(userID, chatID uint) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Check if user is already in the chat room
	if users, exists := h.chatRooms[chatID]; exists {
		if users[userID] {
			// User already in room, skip
			log.Printf("User %d already in chat room %d, skipping join", userID, chatID)
			return
		}
	}

	// Add user to chat room
	if _, exists := h.chatRooms[chatID]; !exists {
		h.chatRooms[chatID] = make(map[uint]bool)
	}
	h.chatRooms[chatID][userID] = true

	// Mark user active in Redis for distributed presence tracking
	h.setUserActiveInChat(userID, chatID)

	// Add chat room to ALL user's connections
	if userConnections, exists := h.clients[userID]; exists {
		for _, client := range userConnections {
			client.mutex.Lock()
			client.chatRooms[chatID] = true
			client.mutex.Unlock()
		}

		log.Printf("User %d joined chat room %d (room has %d users, user has %d connections)",
			userID, chatID, len(h.chatRooms[chatID]), len(userConnections))

		// Notify other users in the chat
		joinData := map[string]interface{}{
			"user_id": userID,
			"chat_id": chatID,
			"action":  "join",
		}

		broadcastMsg := &BroadcastMessage{
			Type:        models.WSMessageTypeUserJoin,
			ChatID:      chatID,
			UserID:      userID,
			Data:        joinData,
			ExcludeUser: userID,
		}

		select {
		case h.broadcast <- broadcastMsg:
		default:
			log.Println("Broadcast channel full, dropping join message")
		}
	}
}

// LeaveChatRoom removes a user from a chat room (updates all user connections)
func (h *Hub) LeaveChatRoom(userID, chatID uint) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	log.Printf("🔴 [LeaveChatRoom] User %d attempting to leave chat room %d", userID, chatID)

	// Remove user active status from Redis
	h.removeUserFromChat(userID, chatID)

	// Remove user from chat room
	if users, exists := h.chatRooms[chatID]; exists {
		wasInRoom := users[userID]
		delete(users, userID)
		if len(users) == 0 {
			delete(h.chatRooms, chatID)
		}

		if wasInRoom {
			log.Printf("✅ [LeaveChatRoom] User %d successfully left chat room %d (room has %d users remaining)", userID, chatID, len(users))
		} else {
			log.Printf("⚠️ [LeaveChatRoom] User %d was NOT in chat room %d", userID, chatID)
		}
	} else {
		log.Printf("⚠️ [LeaveChatRoom] Chat room %d does not exist in chatRooms map", chatID)
	}

	// Remove chat room from ALL user's connections
	if userConnections, exists := h.clients[userID]; exists {
		for _, client := range userConnections {
			client.mutex.Lock()
			delete(client.chatRooms, chatID)
			client.mutex.Unlock()
		}
		log.Printf("✅ [LeaveChatRoom] Removed chat %d from all %d connections of user %d", chatID, len(userConnections), userID)

		// Notify other users in the chat
		leaveData := map[string]interface{}{
			"user_id": userID,
			"chat_id": chatID,
			"action":  "leave",
		}

		broadcastMsg := &BroadcastMessage{
			Type:        models.WSMessageTypeUserLeave,
			ChatID:      chatID,
			UserID:      userID,
			Data:        leaveData,
			ExcludeUser: userID,
		}

		select {
		case h.broadcast <- broadcastMsg:
		default:
			log.Println("Broadcast channel full, dropping leave message")
		}
	} else {
		log.Printf("⚠️ [LeaveChatRoom] User %d not found in h.clients", userID)
	}
}

// BroadcastToChat broadcasts a message to all users in a chat
func (h *Hub) BroadcastToChat(chatID uint, data interface{}, msgType models.WSMessageType, senderID uint) {
	broadcastMsg := &BroadcastMessage{
		Type:        msgType,
		ChatID:      chatID,
		UserID:      senderID,
		Data:        data,
		ExcludeUser: 0, // Send to all users including sender
	}

	select {
	case h.broadcast <- broadcastMsg:
	default:
		log.Println("Broadcast channel is full, dropping message")
	}
}

// BroadcastToChatExcludeSender broadcasts a message to all users in a chat except sender
func (h *Hub) BroadcastToChatExcludeSender(chatID uint, data interface{}, msgType models.WSMessageType, senderID uint) {
	broadcastMsg := &BroadcastMessage{
		Type:        msgType,
		ChatID:      chatID,
		UserID:      senderID,
		Data:        data,
		ExcludeUser: senderID,
	}

	select {
	case h.broadcast <- broadcastMsg:
	default:
		log.Println("Broadcast channel is full, dropping message")
	}
}

// SendToUser sends a message to ALL connections of a specific user (multi-device support)
func (h *Hub) SendToUser(userID uint, data interface{}, msgType models.WSMessageType) {
	h.mutex.RLock()
	userConnections, exists := h.clients[userID]
	h.mutex.RUnlock()

	if !exists || len(userConnections) == 0 {
		log.Printf("User %d not connected, cannot send message", userID)
		return
	}

	message := map[string]interface{}{
		"type":      msgType,
		"data":      data,
		"timestamp": time.Now(),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message for user %d: %v", userID, err)
		return
	}

	// Send to all user's connections
	sent := 0
	for _, client := range userConnections {
		select {
		case client.send <- messageBytes:
			sent++
		default:
			log.Printf("User %d connection send channel full, skipping", userID)
		}
	}

	log.Printf("Sent direct message to user %d (%d connections)", userID, sent)
}

// BroadcastTyping broadcasts typing indicator
func (h *Hub) BroadcastTyping(chatID, userID uint, isTyping bool) {
	typingData := &TypingIndicator{
		UserID:    userID,
		ChatID:    chatID,
		IsTyping:  isTyping,
		Timestamp: time.Now(),
	}

	h.BroadcastToChatExcludeSender(chatID, typingData, models.WSMessageTypeTyping, userID)
}

// GetConnectedUsers returns the list of connected user IDs
func (h *Hub) GetConnectedUsers() []uint {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	users := make([]uint, 0, len(h.clients))
	for userID := range h.clients {
		users = append(users, userID)
	}
	return users
}

// GetChatRoomUsers returns the list of users in a chat room
// Uses Redis for distributed presence tracking if available, falls back to in-memory
func (h *Hub) GetChatRoomUsers(chatID uint) []uint {
	// Try Redis first for distributed tracking
	if h.redisClient != nil {
		userIDs := h.getActiveUsersFromRedis(chatID)
		if len(userIDs) > 0 {
			return userIDs
		}
	}

	// Fallback to in-memory tracking
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if users, exists := h.chatRooms[chatID]; exists {
		userIDs := make([]uint, 0, len(users))
		for userID := range users {
			userIDs = append(userIDs, userID)
		}
		return userIDs
	}
	return []uint{}
}

// GetUserPresence returns user presence info (aggregated from all connections)
func (h *Hub) GetUserPresence(userID uint) *UserPresence {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if userConnections, exists := h.clients[userID]; exists && len(userConnections) > 0 {
		// Aggregate chat rooms from all connections
		chatRoomsMap := make(map[uint]bool)
		var latestSeen time.Time
		status := "offline"

		for _, client := range userConnections {
			// Use the most recent lastSeen
			if client.lastSeen.After(latestSeen) {
				latestSeen = client.lastSeen
				status = client.status
			}
			// Merge chat rooms from all connections
			for chatID := range client.chatRooms {
				chatRoomsMap[chatID] = true
			}
		}

		chatRooms := make([]uint, 0, len(chatRoomsMap))
		for chatID := range chatRoomsMap {
			chatRooms = append(chatRooms, chatID)
		}

		return &UserPresence{
			UserID:       userID,
			Status:       status,
			LastSeen:     latestSeen,
			LastActiveAt: latestSeen,
			ChatRooms:    chatRooms,
		}
	}

	return &UserPresence{
		UserID:       userID,
		Status:       "offline",
		LastSeen:     time.Time{},
		LastActiveAt: time.Time{},
	}
}

// GetMetrics returns current hub metrics
func (h *Hub) GetMetrics() *HubMetrics {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	metrics := *h.metrics
	// ConnectedClients now represents total connections (multi-device)
	metrics.ConnectedClients = h.getTotalConnectionsCount()
	metrics.ActiveChatRooms = len(h.chatRooms)

	return &metrics
}

// updateMetrics periodically updates hub metrics
func (h *Hub) updateMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.mutex.Lock()
			h.metrics.ConnectedClients = h.getTotalConnectionsCount()
			h.metrics.ActiveChatRooms = len(h.chatRooms)
			h.mutex.Unlock()

		case <-h.shutdown:
			return
		}
	}
}

// cleanup cleans up resources on shutdown
func (h *Hub) cleanup() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Close all client connections (multi-device)
	totalClosed := 0
	for userID, userConnections := range h.clients {
		for connID, client := range userConnections {
			close(client.send)
			client.conn.Close()
			log.Printf("Closed connection %s for user %d", connID[:8], userID)
			totalClosed++
		}
	}

	// Clear all data structures
	h.clients = make(map[uint]map[string]*Client)
	h.chatRooms = make(map[uint]map[uint]bool)

	log.Printf("Hub cleanup completed (%d connections closed)", totalClosed)
}

// RegisterClient registers a client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client from the hub
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Redis-based presence tracking methods

// setUserActiveInChat marks a user as active in a specific chat using Redis
func (h *Hub) setUserActiveInChat(userID, chatID uint) {
	if h.redisClient == nil {
		return
	}

	key := fmt.Sprintf("chat:active_user:%d:%d", chatID, userID)
	// Set with 30 second TTL - will be refreshed by heartbeat/ping
	err := h.redisClient.Set(key, "1", 30*time.Second)
	if err != nil {
		log.Printf("Failed to set user %d active in chat %d in Redis: %v", userID, chatID, err)
	}
}

// removeUserFromChat removes a user's active status from a chat in Redis
func (h *Hub) removeUserFromChat(userID, chatID uint) {
	if h.redisClient == nil {
		return
	}

	key := fmt.Sprintf("chat:active_user:%d:%d", chatID, userID)
	err := h.redisClient.Delete(key)
	if err != nil {
		log.Printf("Failed to remove user %d from chat %d in Redis: %v", userID, chatID, err)
	}
}

// getActiveUsersFromRedis gets the list of active users in a chat from Redis
func (h *Hub) getActiveUsersFromRedis(chatID uint) []uint {
	if h.redisClient == nil {
		return []uint{}
	}

	// Get all keys matching the pattern chat:active_user:chatID:*
	pattern := fmt.Sprintf("chat:active_user:%d:*", chatID)

	// Use the underlying Redis client to call Keys with background context
	ctx := context.Background()
	keys, err := h.redisClient.Client.Keys(ctx, pattern).Result()
	if err != nil {
		log.Printf("Failed to get active users for chat %d from Redis: %v", chatID, err)
		return []uint{}
	}

	userIDs := make([]uint, 0, len(keys))
	for _, key := range keys {
		// Extract userID from key: chat:active_user:chatID:userID
		var userID uint
		if _, err := fmt.Sscanf(key, fmt.Sprintf("chat:active_user:%d:%%d", chatID), &userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}

	return userIDs
}

// refreshUserPresence refreshes a user's presence in all their active chats (all connections)
func (h *Hub) refreshUserPresence(userID uint) {
	if h.redisClient == nil {
		return
	}

	// Get the user's active chat rooms from all connections
	h.mutex.RLock()
	userConnections, exists := h.clients[userID]
	h.mutex.RUnlock()

	if !exists || len(userConnections) == 0 {
		return
	}

	// Collect all unique chat rooms from all connections
	chatRoomsMap := make(map[uint]bool)
	for _, client := range userConnections {
		client.mutex.RLock()
		for chatID := range client.chatRooms {
			chatRoomsMap[chatID] = true
		}
		client.mutex.RUnlock()
	}

	// Refresh presence in all active chat rooms
	for chatID := range chatRoomsMap {
		h.setUserActiveInChat(userID, chatID)
	}
}

// resetStuckOnlineStatuses resets all online statuses to offline on startup
// This ensures that users who were stuck in "online" status due to service crashes
// or network issues are properly reset to offline
func (h *Hub) resetStuckOnlineStatuses() {
	log.Println("🔄 Resetting stuck online statuses on startup...")

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8081"
	}

	url := fmt.Sprintf("%s/internal/users/reset-online-statuses", userServiceURL)

	// Create request with context for timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		log.Printf("❌ Failed to create reset online statuses request: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to reset online statuses: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ User service returned non-OK status %d when resetting online statuses", resp.StatusCode)
		return
	}

	log.Println("✅ Successfully reset all stuck online statuses to offline")
}

// periodicStatusCleanup periodically checks for inactive users and resets their status to offline
// This runs every 5 minutes and checks if any user in "online" status is not actually connected
func (h *Hub) periodicStatusCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Println("🔄 Started periodic status cleanup (every 5 minutes)")

	for {
		select {
		case <-ticker.C:
			h.cleanupInactiveStatuses()

		case <-h.shutdown:
			log.Println("⏹️ Stopped periodic status cleanup")
			return
		}
	}
}

// cleanupInactiveStatuses checks for users marked as online but not connected to WebSocket
func (h *Hub) cleanupInactiveStatuses() {
	log.Println("🔍 Running inactive status cleanup...")

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8081"
	}

	// Get list of currently connected users
	h.mutex.RLock()
	connectedUserIDs := make([]uint, 0, len(h.clients))
	for userID := range h.clients {
		connectedUserIDs = append(connectedUserIDs, userID)
	}
	h.mutex.RUnlock()

	// Send list of connected users to user-service
	// User-service will mark all other "online" users as offline
	payload := map[string]interface{}{
		"connected_user_ids": connectedUserIDs,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Failed to marshal connected users: %v", err)
		return
	}

	url := fmt.Sprintf("%s/internal/users/cleanup-statuses", userServiceURL)

	// Create request with context for timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create cleanup statuses request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to cleanup statuses: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ User service returned non-OK status %d when cleaning up statuses", resp.StatusCode)
		return
	}

	log.Printf("✅ Status cleanup completed (%d users currently connected)", len(connectedUserIDs))
}
