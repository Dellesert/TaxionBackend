package websocket

import (
	"encoding/json"
	"log"
	"time"

	"tachyon-messenger/services/chat/models"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192
)

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, hub *Hub, userID uint) *Client {
	return &Client{
		conn:      conn,
		send:      make(chan []byte, 512),
		hub:       hub,
		userID:    userID,
		chatRooms: make(map[uint]bool),
		lastSeen:  time.Now(),
		status:    "online",
	}
}

// ReadPump pumps messages from the websocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
		log.Printf("ReadPump stopped for user %d", c.userID)
	}()

	// Set connection limits and handlers
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
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

	default:
		log.Printf("Unknown message type %s from user %d", wsMsg.Type, c.userID)
		c.sendErrorMessage("Unknown message type")
	}
}

// handleTypingMessage handles typing indicator messages
func (c *Client) handleTypingMessage(wsMsg *models.WSMessage) {
	if typingData, ok := wsMsg.Data.(map[string]interface{}); ok {
		if isTyping, exists := typingData["is_typing"].(bool); exists {
			c.hub.BroadcastTyping(wsMsg.ChatID, c.userID, isTyping)
		}
	}
}

// handleJoinMessage handles user join messages
func (c *Client) handleJoinMessage(wsMsg *models.WSMessage) {
	c.joinChatRoom(wsMsg.ChatID)
	log.Printf("User %d joined chat room %d", c.userID, wsMsg.ChatID)
}

// handleLeaveMessage handles user leave messages
func (c *Client) handleLeaveMessage(wsMsg *models.WSMessage) {
	c.leaveChatRoom(wsMsg.ChatID)
	log.Printf("User %d left chat room %d", c.userID, wsMsg.ChatID)
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
	// Forward the message to the hub for broadcasting
	if chatData, ok := wsMsg.Data.(map[string]interface{}); ok {
		log.Printf("Chat message from user %d in chat %d: %v", c.userID, wsMsg.ChatID, chatData)
		// Here you can implement chat message processing logic
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
