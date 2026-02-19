package websocket

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"tachyon-messenger/services/chat/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 90 * time.Second // Increased from 60s to 90s for better stability

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10 // 81 seconds

	// Maximum message size allowed from peer
	maxMessageSize = 8192
)

// generateConnectionID creates a unique connection ID for multi-device support
func generateConnectionID() string {
	return uuid.New().String()
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, hub *Hub, userID uint, sessionID string) *Client {
	return &Client{
		conn:         conn,
		send:         make(chan []byte, 512),
		hub:          hub,
		userID:       userID,
		sessionID:    sessionID,
		connectionID: generateConnectionID(),
		chatRooms:    make(map[uint]bool),
		lastSeen:     time.Now(),
		status:       "online",
	}
}

// NewClientWithConnectionID creates a new WebSocket client with a specific connection ID
// Used when client provides their own device/connection identifier
func NewClientWithConnectionID(conn *websocket.Conn, hub *Hub, userID uint, sessionID string, connectionID string) *Client {
	if connectionID == "" {
		connectionID = generateConnectionID()
	}
	return &Client{
		conn:         conn,
		send:         make(chan []byte, 512),
		hub:          hub,
		userID:       userID,
		sessionID:    sessionID,
		connectionID: connectionID,
		chatRooms:    make(map[uint]bool),
		lastSeen:     time.Now(),
		status:       "online",
	}
}

// ReadPump pumps messages from the websocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		log.Printf("🔌 WebSocket disconnecting for user %d", c.userID)
		c.hub.UnregisterClient(c)
		c.conn.Close()
		log.Printf("✅ ReadPump stopped and user %d unregistered", c.userID)
	}()

	// Set connection limits and handlers
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		log.Printf("✅ Received pong from user %d - connection alive", c.userID)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.updateLastSeen()
		return nil
	})

	for {
		// Read message from WebSocket
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for user %d: %v", c.userID, err)
			}
			break
		}

		// Update metrics and last seen
		c.hub.metrics.MessagesReceived++
		c.updateLastSeen()

		// Handle the message
		c.handleIncomingMessage(messageBytes)
	}
}

// WritePump pumps messages from the hub to the websocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		log.Printf("WritePump stopped for user %d", c.userID)
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Get writer for text message
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Error getting writer for user %d: %v", c.userID, err)
				return
			}

			// Write the message
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			// Close the writer
			if err := w.Close(); err != nil {
				log.Printf("Error closing writer for user %d: %v", c.userID, err)
				return
			}

		case <-ticker.C:
			// Send ping message for keep-alive
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Error sending ping to user %d: %v", c.userID, err)
				return
			}
		}
	}
}

// handleIncomingMessage handles incoming WebSocket messages
func (c *Client) handleIncomingMessage(messageBytes []byte) {
	// Parse incoming message
	var wsMsg models.WSMessage
	if err := json.Unmarshal(messageBytes, &wsMsg); err != nil {
		log.Printf("Error parsing WebSocket message from user %d: %v", c.userID, err)
		c.sendErrorMessage("Invalid message format")
		return
	}

	// Set user ID and timestamp
	wsMsg.UserID = c.userID
	wsMsg.SentAt = time.Now()

	// Handle different message types
	c.handleWebSocketMessage(&wsMsg)
}

// handleWebSocketMessage handles different types of WebSocket messages
func (c *Client) handleWebSocketMessage(wsMsg *models.WSMessage) {
	switch wsMsg.Type {
	case models.WSMessageTypeTyping:
		c.handleTypingMessage(wsMsg)

	case models.WSMessageTypeUserJoin:
		c.handleJoinMessage(wsMsg)

	case models.WSMessageTypeUserLeave:
		c.handleLeaveMessage(wsMsg)

	case models.WSMessageTypeRead:
		c.handleReadMessage(wsMsg)

	case models.WSMessageTypeNewMessage:
		c.handleChatMessage(wsMsg)

	case models.WSMessageTypeUserPresence:
		c.handlePresenceMessage(wsMsg)

	default:
		log.Printf("Unknown message type %s from user %d", wsMsg.Type, c.userID)
		c.sendErrorMessage("Unknown message type")
	}
}

// handleTypingMessage handles typing indicator messages
func (c *Client) handleTypingMessage(wsMsg *models.WSMessage) {
	if typingData, ok := wsMsg.Data.(map[string]interface{}); ok {
		if isTyping, exists := typingData["is_typing"].(bool); exists {
			action, _ := typingData["action"].(string)
			if action == "" {
				action = "typing"
			}
			c.hub.BroadcastTyping(wsMsg.ChatID, c.userID, isTyping, action)
		}
	}
}

// handleJoinMessage handles user join messages
func (c *Client) handleJoinMessage(wsMsg *models.WSMessage) {
	c.joinChatRoom(wsMsg.ChatID)
	log.Printf("User %d joined chat room %d", c.userID, wsMsg.ChatID)

	// Note: user presence is broadcasted only once during client registration (see hub.registerClient)
	// We don't broadcast presence on every join to avoid spamming presence updates
}

// handleLeaveMessage handles user leave messages
func (c *Client) handleLeaveMessage(wsMsg *models.WSMessage) {
	// FIXED: Re-enabled leave handling now that frontend properly sends join/leave events
	log.Printf("User %d leaving chat room %d", c.userID, wsMsg.ChatID)
	c.leaveChatRoom(wsMsg.ChatID)
}

// handlePresenceMessage handles user presence status changes (online/away/offline)
// This is called when client sends user_presence message (e.g., app goes to background)
func (c *Client) handlePresenceMessage(wsMsg *models.WSMessage) {
	if presenceData, ok := wsMsg.Data.(map[string]interface{}); ok {
		if status, exists := presenceData["status"].(string); exists {
			// Validate status
			if status != "online" && status != "away" && status != "offline" {
				log.Printf("Invalid presence status '%s' from user %d", status, c.userID)
				return
			}

			log.Printf("User %d presence changed to: %s", c.userID, status)

			// Update client status
			c.SetStatus(status)

			// Broadcast presence change to other users in the same chats
			c.hub.broadcastUserPresence(c.userID, status)
		}
	}
}

// handleReadMessage handles message read notifications
func (c *Client) handleReadMessage(wsMsg *models.WSMessage) {
	if readData, ok := wsMsg.Data.(map[string]interface{}); ok {
		if messageID, exists := readData["message_id"]; exists {
			log.Printf("User %d marked message %v as read in chat %d", c.userID, messageID, wsMsg.ChatID)
			// Here you can implement message read tracking logic
		}
	}
}

// handleChatMessage handles chat messages
func (c *Client) handleChatMessage(wsMsg *models.WSMessage) {
	// Validate message data
	chatData, ok := wsMsg.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid chat message data from user %d", c.userID)
		c.sendErrorMessage("Invalid message data")
		return
	}

	// Extract content from message data
	content, exists := chatData["content"].(string)
	if !exists || strings.TrimSpace(content) == "" {
		log.Printf("Missing or empty content in message from user %d", c.userID)
		c.sendErrorMessage("Message content is required")
		return
	}

	// Extract message type (optional)
	messageType := "text"
	if msgType, exists := chatData["type"].(string); exists {
		messageType = msgType
	}

	log.Printf("Chat message from user %d in chat %d: %s", c.userID, wsMsg.ChatID, content)

	// ВАЖНО: Сохраняем сообщение в базу данных через MessageUsecase
	// Создаем request для сохранения сообщения
	sendRequest := &models.SendMessageRequest{
		ChatID:  wsMsg.ChatID,
		Content: content,
		Type:    models.MessageType(messageType),
	}

	// Получаем доступ к messageUsecase через хаб
	if c.hub.messageUsecase != nil {
		// Сохраняем сообщение в БД
		// ВАЖНО: SendMessage() уже делает broadcast через WebSocket hub с полным MessageResponse
		// Поэтому здесь НЕ нужно делать дополнительный broadcast
		savedMessage, err := c.hub.messageUsecase.SendMessage(c.userID, sendRequest)
		if err != nil {
			log.Printf("Failed to save message to database for user %d: %v", c.userID, err)
			c.sendErrorMessage("Failed to save message")
			return
		}

		// НЕ делаем broadcast здесь - SendMessage() уже это сделал!
		// Это исправляет проблему с пустым content в WebSocket сообщениях
		log.Printf("✅ Message saved to database with ID %d (broadcast handled by SendMessage)", savedMessage.ID)
	} else {
		// Fallback: если messageUsecase недоступен, просто broadcast без сохранения
		log.Printf("MessageUsecase not available, broadcasting without saving to database")
		c.hub.BroadcastToChat(wsMsg.ChatID, chatData, models.WSMessageTypeNewMessage, c.userID)
	}
}

// sendErrorMessage sends an error message to the client
func (c *Client) sendErrorMessage(errorMsg string) {
	// Создаем inline константу или используем строку напрямую
	const WSMessageTypeError = "error"

	errorMessage := map[string]interface{}{
		"type":      WSMessageTypeError,
		"data":      map[string]string{"error": errorMsg},
		"timestamp": time.Now(),
	}

	if messageBytes, err := json.Marshal(errorMessage); err == nil {
		select {
		case c.send <- messageBytes:
		default:
			log.Printf("Error message could not be sent to user %d: channel full", c.userID)
		}
	}
}

// joinChatRoom adds the client to a chat room
func (c *Client) joinChatRoom(chatID uint) {
	c.mutex.Lock()
	c.chatRooms[chatID] = true
	c.mutex.Unlock()

	// Notify hub about the join
	c.hub.JoinChatRoom(c.userID, chatID)
}

// leaveChatRoom removes the client from a chat room
func (c *Client) leaveChatRoom(chatID uint) {
	c.mutex.Lock()
	delete(c.chatRooms, chatID)
	c.mutex.Unlock()

	// Notify hub about the leave
	c.hub.LeaveChatRoom(c.userID, chatID)
}

// updateLastSeen updates the client's last seen timestamp
func (c *Client) updateLastSeen() {
	c.lastSeen = time.Now()
}

// GetStatus returns the current status of the client
func (c *Client) GetStatus() string {
	return c.status
}

// SetStatus sets the client's status
func (c *Client) SetStatus(status string) {
	c.status = status
}

// GetLastSeen returns the last seen timestamp
func (c *Client) GetLastSeen() time.Time {
	return c.lastSeen
}

// GetChatRooms returns a copy of the client's chat rooms
func (c *Client) GetChatRooms() map[uint]bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	rooms := make(map[uint]bool)
	for chatID, active := range c.chatRooms {
		rooms[chatID] = active
	}
	return rooms
}

// IsInChatRoom checks if the client is in a specific chat room
func (c *Client) IsInChatRoom(chatID uint) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.chatRooms[chatID]
}

// Close gracefully closes the client connection
func (c *Client) Close() {
	c.conn.Close()
}

// GetConnectionID returns the unique connection ID
func (c *Client) GetConnectionID() string {
	return c.connectionID
}

// GetUserID returns the user ID
func (c *Client) GetUserID() uint {
	return c.userID
}

// GetSessionID returns the session ID
func (c *Client) GetSessionID() string {
	return c.sessionID
}
