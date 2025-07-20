package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"tachyon-messenger/services/chat/models"

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
	UserID    uint      `json:"user_id"`
	Status    string    `json:"status"` // online, away, busy, offline
	LastSeen  time.Time `json:"last_seen"`
	ChatRooms []uint    `json:"chat_rooms,omitempty"`
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uint]*Client),
		chatRooms:  make(map[uint]map[uint]bool),
		broadcast:  make(chan *BroadcastMessage, 1024),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shutdown:   make(chan struct{}),
		metrics: &HubMetrics{
			Uptime: time.Now(),
		},
	}
}

// Run starts the hub and handles client connections
func (h *Hub) Run() {
	log.Println("WebSocket hub started")

	// Start metrics updater
	go h.updateMetrics()

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

// registerClient registers a new client
func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Close existing connection if user is already connected
	if existingClient, exists := h.clients[client.userID]; exists {
		log.Printf("Replacing existing connection for user %d", client.userID)
		h.forceDisconnectClient(existingClient)
	}

	h.clients[client.userID] = client
	client.status = "online"
	client.lastSeen = time.Now()

	log.Printf("Client registered: user %d (total clients: %d)", client.userID, len(h.clients))

	// Notify about user coming online
	h.broadcastUserPresence(client.userID, "online")
}

// unregisterClient unregisters a client
func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if storedClient, exists := h.clients[client.userID]; exists && storedClient == client {
		delete(h.clients, client.userID)
		close(client.send)

		// Remove user from all chat rooms
		for chatID := range client.chatRooms {
			if users, exists := h.chatRooms[chatID]; exists {
				delete(users, client.userID)
				if len(users) == 0 {
					delete(h.chatRooms, chatID)
				}
			}
		}

		log.Printf("Client unregistered: user %d (remaining clients: %d)", client.userID, len(h.clients))

		// Notify about user going offline
		h.broadcastUserPresence(client.userID, "offline")
	}
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

// broadcastMessage broadcasts a message to relevant clients
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

	// Get users in the chat room
	if users, exists := h.chatRooms[broadcastMsg.ChatID]; exists {
		for userID := range users {
			// Skip excluded user (e.g., message sender)
			if broadcastMsg.ExcludeUser != 0 && userID == broadcastMsg.ExcludeUser {
				continue
			}

			if client, clientExists := h.clients[userID]; clientExists {
				select {
				case client.send <- message:
					sent++
				default:
					// Client's send channel is full, remove client
					log.Printf("Client %d send channel full, removing", userID)
					close(client.send)
					delete(h.clients, userID)
					delete(users, userID)
				}
			}
		}
	}

	h.metrics.MessagesSent += int64(sent)

	if sent > 0 {
		log.Printf("Broadcasted %s message to %d clients in chat %d", broadcastMsg.Type, sent, broadcastMsg.ChatID)
	}
}

// broadcastUserPresence broadcasts user presence change
func (h *Hub) broadcastUserPresence(userID uint, status string) {
	client := h.clients[userID]
	if client == nil {
		return
	}

	presence := &UserPresence{
		UserID:   userID,
		Status:   status,
		LastSeen: time.Now(),
	}

	// Get user's chat rooms
	chatRooms := make([]uint, 0, len(client.chatRooms))
	for chatID := range client.chatRooms {
		chatRooms = append(chatRooms, chatID)
	}
	presence.ChatRooms = chatRooms

	// Broadcast to all chat rooms user is in
	for chatID := range client.chatRooms {
		broadcastMsg := &BroadcastMessage{
			Type:        models.WSMessageType("user_presence"),
			ChatID:      chatID,
			UserID:      userID,
			Data:        presence,
			ExcludeUser: userID, // Don't send to the user themselves
		}

		select {
		case h.broadcast <- broadcastMsg:
		default:
			log.Println("Broadcast channel full, dropping presence message")
		}
	}
}

// JoinChatRoom adds a user to a chat room
func (h *Hub) JoinChatRoom(userID, chatID uint) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Add user to chat room
	if _, exists := h.chatRooms[chatID]; !exists {
		h.chatRooms[chatID] = make(map[uint]bool)
	}
	h.chatRooms[chatID][userID] = true

	// Add chat room to client
	if client, exists := h.clients[userID]; exists {
		client.mutex.Lock()
		client.chatRooms[chatID] = true
		client.mutex.Unlock()

		log.Printf("User %d joined chat room %d (room has %d users)", userID, chatID, len(h.chatRooms[chatID]))

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

// LeaveChatRoom removes a user from a chat room
func (h *Hub) LeaveChatRoom(userID, chatID uint) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Remove user from chat room
	if users, exists := h.chatRooms[chatID]; exists {
		delete(users, userID)
		if len(users) == 0 {
			delete(h.chatRooms, chatID)
		}

		log.Printf("User %d left chat room %d (room has %d users)", userID, chatID, len(users))
	}

	// Remove chat room from client
	if client, exists := h.clients[userID]; exists {
		client.mutex.Lock()
		delete(client.chatRooms, chatID)
		client.mutex.Unlock()

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

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID uint, data interface{}, msgType models.WSMessageType) {
	h.mutex.RLock()
	client, exists := h.clients[userID]
	h.mutex.RUnlock()

	if !exists {
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

	select {
	case client.send <- messageBytes:
		log.Printf("Sent direct message to user %d", userID)
	default:
		// Client's send channel is full, remove client
		h.mutex.Lock()
		close(client.send)
		delete(h.clients, userID)
		h.mutex.Unlock()
		log.Printf("User %d send channel full, client removed", userID)
	}
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
func (h *Hub) GetChatRoomUsers(chatID uint) []uint {
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

// GetUserPresence returns user presence info
func (h *Hub) GetUserPresence(userID uint) *UserPresence {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if client, exists := h.clients[userID]; exists {
		chatRooms := make([]uint, 0, len(client.chatRooms))
		for chatID := range client.chatRooms {
			chatRooms = append(chatRooms, chatID)
		}

		return &UserPresence{
			UserID:    userID,
			Status:    client.status,
			LastSeen:  client.lastSeen,
			ChatRooms: chatRooms,
		}
	}

	return &UserPresence{
		UserID:   userID,
		Status:   "offline",
		LastSeen: time.Time{},
	}
}

// GetMetrics returns current hub metrics
func (h *Hub) GetMetrics() *HubMetrics {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	metrics := *h.metrics
	metrics.ConnectedClients = len(h.clients)
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
			h.metrics.ConnectedClients = len(h.clients)
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

	// Close all client connections
	for userID, client := range h.clients {
		close(client.send)
		client.conn.Close()
		log.Printf("Closed connection for user %d", userID)
	}

	// Clear all data structures
	h.clients = make(map[uint]*Client)
	h.chatRooms = make(map[uint]map[uint]bool)

	log.Println("Hub cleanup completed")
}

// RegisterClient registers a client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client from the hub
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}
