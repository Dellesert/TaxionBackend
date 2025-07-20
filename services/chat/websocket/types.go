package websocket

import (
	"sync"
	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/usecase"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MessageTypeChat   MessageType = "chat"
	MessageTypeTyping MessageType = "typing"
	MessageTypeJoin   MessageType = "join"
	MessageTypeLeave  MessageType = "leave"
	MessageTypeError  MessageType = "error"
	MessageTypeStatus MessageType = "status"
	MessageTypePing   MessageType = "ping"
	MessageTypePong   MessageType = "pong"
)

// ConnectionStatus represents the status of a WebSocket connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusReconnecting ConnectionStatus = "reconnecting"
)

// Client represents a websocket client
type Client struct {
	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// The hub that manages this client
	hub *Hub

	// User ID
	userID uint

	// Chat rooms the client is subscribed to
	chatRooms map[uint]bool

	// Mutex for thread-safe access to chatRooms
	mutex sync.RWMutex

	// Last seen timestamp for presence tracking
	lastSeen time.Time

	// Client status (online, away, busy, offline)
	status string
}

// Hub maintains the set of active clients and broadcasts messages to clients

// Hub manages all WebSocket client connections
type Hub struct {
	// Registered clients mapped by user ID
	clients map[uint]*Client

	// Chat rooms - maps chat ID to set of user IDs
	chatRooms map[uint]map[uint]bool

	// Inbound messages from the clients
	broadcast chan *BroadcastMessage

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe access
	mutex sync.RWMutex

	// Channel to signal hub shutdown
	shutdown chan struct{}

	// Metrics for monitoring
	metrics *HubMetrics

	messageUsecase usecase.MessageUsecase
}

// BroadcastMessage represents a message to be broadcasted to a room
// BroadcastMessage represents a message to be broadcasted
type BroadcastMessage struct {
	Type        models.WSMessageType `json:"type"`
	ChatID      uint                 `json:"chat_id"`
	UserID      uint                 `json:"user_id,omitempty"`
	Data        interface{}          `json:"data"`
	Timestamp   time.Time            `json:"timestamp"`
	ExcludeUser uint                 `json:"-"` // Don't send to this user
}

// DirectMessage represents a message to be sent to a specific client
type DirectMessage struct {
	// Target client ID
	ClientID string `json:"client_id"`

	// Message type
	Type MessageType `json:"type"`

	// Message payload
	Data interface{} `json:"data"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// Message represents a WebSocket message structure
type Message struct {
	// Message type
	Type MessageType `json:"type"`

	// Message payload
	Data interface{} `json:"data"`

	// Metadata
	Meta *MessageMeta `json:"meta,omitempty"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// MessageMeta contains metadata about the message
type MessageMeta struct {
	// Message ID for tracking
	ID string `json:"id,omitempty"`

	// Room ID where message should be sent
	RoomID uint `json:"room_id,omitempty"`

	// Sender information
	SenderID   string `json:"sender_id,omitempty"`
	SenderName string `json:"sender_name,omitempty"`

	// Message priority
	Priority int `json:"priority,omitempty"`

	// TTL for message expiration
	TTL time.Duration `json:"ttl,omitempty"`

	// Delivery options
	RequireAck bool `json:"require_ack,omitempty"`
}

// HubStats contains statistics about the hub
type HubStats struct {
	// Number of connected clients
	ConnectedClients int `json:"connected_clients"`

	// Number of active rooms
	ActiveRooms int `json:"active_rooms"`

	// Total messages sent
	MessagesSent int64 `json:"messages_sent"`

	// Total messages received
	MessagesReceived int64 `json:"messages_received"`

	// Hub uptime
	Uptime time.Duration `json:"uptime"`

	// Start time
	StartTime time.Time `json:"start_time"`

	// Mutex for thread-safe access to stats
	Mutex sync.RWMutex `json:"-"`
}

// ConnectionManager defines the interface for managing WebSocket connections
type ConnectionManager interface {
	// Client management
	RegisterClient(client *Client) error
	UnregisterClient(clientID string) error
	GetClient(clientID string) (*Client, bool)
	GetClientByUserID(userID uint) (*Client, bool)
	GetAllClients() []*Client

	// Room management
	JoinRoom(clientID string, roomID uint) error
	LeaveRoom(clientID string, roomID uint) error
	GetRoomClients(roomID uint) []*Client
	GetClientRooms(clientID string) []uint

	// Message handling
	BroadcastToRoom(roomID uint, message *Message) error
	BroadcastToAll(message *Message) error
	SendToClient(clientID string, message *Message) error
	SendToUser(userID uint, message *Message) error

	// Hub management
	Start() error
	Stop() error
	GetStats() *HubStats
	IsRunning() bool
}

// ChatMessage represents a chat message sent through WebSocket
type ChatMessage struct {
	// Chat ID where message belongs
	ChatID uint `json:"chat_id"`

	// Message content
	Content string `json:"content"`

	// Message type (text, image, file, etc.)
	MessageType string `json:"message_type"`

	// Sender user ID
	SenderID uint `json:"sender_id"`

	// Reply to message ID (optional)
	ReplyToID *uint `json:"reply_to_id,omitempty"`

	// File information (for file messages)
	FileInfo *FileInfo `json:"file_info,omitempty"`
}

// TypingMessage represents a typing indicator message
type TypingMessage struct {
	// Chat ID where user is typing
	ChatID uint `json:"chat_id"`

	// User ID who is typing
	UserID uint `json:"user_id"`

	// Whether user is currently typing
	IsTyping bool `json:"is_typing"`
}

// JoinMessage represents a room join message
type JoinMessage struct {
	// Room ID to join
	RoomID uint `json:"room_id"`

	// User ID joining the room
	UserID uint `json:"user_id"`
}

// LeaveMessage represents a room leave message
type LeaveMessage struct {
	// Room ID to leave
	RoomID uint `json:"room_id"`

	// User ID leaving the room
	UserID uint `json:"user_id"`
}

// StatusMessage represents a status update message
type StatusMessage struct {
	// User ID
	UserID uint `json:"user_id"`

	// Online status
	Online bool `json:"online"`

	// Last seen timestamp
	LastSeen time.Time `json:"last_seen"`
}

// ErrorMessage represents an error message
type ErrorMessage struct {
	// Error code
	Code string `json:"code"`

	// Error message
	Message string `json:"message"`

	// Additional error details
	Details interface{} `json:"details,omitempty"`
}

// FileInfo represents file information for file messages
type FileInfo struct {
	// File name
	Name string `json:"name"`

	// File size in bytes
	Size int64 `json:"size"`

	// MIME type
	MimeType string `json:"mime_type"`

	// File URL
	URL string `json:"url"`

	// Thumbnail URL (for images/videos)
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// ClientOptions represents options for creating a new client
type ClientOptions struct {
	// Buffer size for send channel
	SendBufferSize int

	// Read/Write timeouts
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Maximum message size
	MaxMessageSize int64

	// Ping interval
	PingInterval time.Duration

	// Pong timeout
	PongTimeout time.Duration
}

// HubOptions represents options for creating a new hub
type HubOptions struct {
	// Buffer sizes for channels
	RegisterBufferSize   int
	UnregisterBufferSize int
	BroadcastBufferSize  int

	// Statistics update interval
	StatsInterval time.Duration

	// Cleanup interval for inactive connections
	CleanupInterval time.Duration

	// Maximum clients per room
	MaxClientsPerRoom int

	// Maximum rooms per client
	MaxRoomsPerClient int
}

// DefaultClientOptions returns default options for WebSocket clients
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		SendBufferSize: 256,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxMessageSize: 8192,
		PingInterval:   54 * time.Second,
		PongTimeout:    60 * time.Second,
	}
}

// DefaultHubOptions returns default options for WebSocket hub
func DefaultHubOptions() *HubOptions {
	return &HubOptions{
		RegisterBufferSize:   256,
		UnregisterBufferSize: 256,
		BroadcastBufferSize:  1024,
		StatsInterval:        30 * time.Second,
		CleanupInterval:      5 * time.Minute,
		MaxClientsPerRoom:    1000,
		MaxRoomsPerClient:    100,
	}
}
