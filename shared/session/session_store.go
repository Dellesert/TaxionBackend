package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"tachyon-messenger/shared/models"

	"github.com/redis/go-redis/v9"
)

// SessionStore manages user sessions in Redis
type SessionStore struct {
	client             *redis.Client
	sessionDuration    time.Duration
	maxSessionsPerUser int
}

// NewSessionStore creates a new session store
func NewSessionStore(client *redis.Client, sessionDuration time.Duration) *SessionStore {
	if sessionDuration == 0 {
		sessionDuration = 7 * 24 * time.Hour // Default 7 days
	}
	return &SessionStore{
		client:             client,
		sessionDuration:    sessionDuration,
		maxSessionsPerUser: 5, // Max 5 concurrent sessions per user
	}
}

// UpdateSessionDuration updates the session duration dynamically
func (s *SessionStore) UpdateSessionDuration(newDuration time.Duration) {
	if newDuration > 0 {
		s.sessionDuration = newDuration
	}
}

// GetSessionDuration returns current session duration
func (s *SessionStore) GetSessionDuration() time.Duration {
	return s.sessionDuration
}

// UpdateMaxSessionsPerUser updates the max sessions per user dynamically
func (s *SessionStore) UpdateMaxSessionsPerUser(n int) {
	if n > 0 {
		s.maxSessionsPerUser = n
	}
}

// GetMaxSessionsPerUser returns current max sessions per user
func (s *SessionStore) GetMaxSessionsPerUser() int {
	return s.maxSessionsPerUser
}

// CreateSession creates a new session for a user.
// Returns the new session, a list of evicted session IDs (if the limit was exceeded), and an error.
func (s *SessionStore) CreateSession(ctx context.Context, userID uint, email string, role models.Role, ipAddress, userAgent string) (*models.Session, []string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(s.sessionDuration)

	session := &models.Session{
		SessionID:    sessionID,
		UserID:       userID,
		Email:        email,
		Role:         role,
		DepartmentID: nil, // Will be set by middleware on retrieval
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastActiveAt: now,
	}

	// Store session in Redis
	sessionKey := sessionKey(sessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	err = s.client.Set(ctx, sessionKey, sessionData, s.sessionDuration).Err()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store session in Redis: %w", err)
	}

	// Add to user's session list
	userSessionsKey := userSessionsKey(userID)
	err = s.client.ZAdd(ctx, userSessionsKey, redis.Z{
		Score:  float64(now.Unix()),
		Member: sessionID,
	}).Err()
	if err != nil {
		// Clean up session if we can't add to user list
		s.client.Del(ctx, sessionKey)
		return nil, nil, fmt.Errorf("failed to add session to user list: %w", err)
	}

	// Set expiration for user sessions list
	s.client.Expire(ctx, userSessionsKey, s.sessionDuration+24*time.Hour)

	// Limit concurrent sessions per user
	evictedSessionIDs := s.limitUserSessions(ctx, userID)

	return session, evictedSessionIDs, nil
}

// GetSession retrieves a session by session ID
func (s *SessionStore) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	sessionKey := sessionKey(sessionID)
	sessionData, err := s.client.Get(ctx, sessionKey).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session from Redis: %w", err)
	}

	var session models.Session
	err = json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		s.DeleteSession(ctx, sessionID)
		return nil, fmt.Errorf("session expired")
	}

	return &session, nil
}

// UpdateSessionActivity updates the last active time of a session and extends its expiration
func (s *SessionStore) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	now := time.Now()
	session.LastActiveAt = now
	// Продлить сессию на полный срок с момента последней активности
	session.ExpiresAt = now.Add(s.sessionDuration)

	sessionKey := sessionKey(sessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Update session with full session duration (sliding window)
	err = s.client.Set(ctx, sessionKey, sessionData, s.sessionDuration).Err()
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// DeleteSession deletes a session by session ID
func (s *SessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	// Get session to find user ID
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		// Session might not exist, which is fine
		sessionKey := sessionKey(sessionID)
		s.client.Del(ctx, sessionKey)
		return nil
	}

	// Delete session
	sessionKey := sessionKey(sessionID)
	err = s.client.Del(ctx, sessionKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove from user's session list
	userSessionsKey := userSessionsKey(session.UserID)
	s.client.ZRem(ctx, userSessionsKey, sessionID)

	return nil
}

// DeleteUserSessions deletes all sessions for a user
func (s *SessionStore) DeleteUserSessions(ctx context.Context, userID uint) error {
	userSessionsKey := userSessionsKey(userID)

	// Get all session IDs for user
	sessionIDs, err := s.client.ZRange(ctx, userSessionsKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Delete all sessions
	for _, sessionID := range sessionIDs {
		sessionKey := sessionKey(sessionID)
		s.client.Del(ctx, sessionKey)
	}

	// Delete user sessions list
	s.client.Del(ctx, userSessionsKey)

	return nil
}

// GetUserSessions gets all active sessions for a user
func (s *SessionStore) GetUserSessions(ctx context.Context, userID uint) ([]*models.Session, error) {
	userSessionsKey := userSessionsKey(userID)

	// Get all session IDs for user
	sessionIDs, err := s.client.ZRange(ctx, userSessionsKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	sessions := make([]*models.Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := s.GetSession(ctx, sessionID)
		if err != nil {
			// Skip invalid/expired sessions
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// limitUserSessions limits the number of concurrent sessions per user.
// Returns the list of evicted session IDs.
func (s *SessionStore) limitUserSessions(ctx context.Context, userID uint) []string {
	userSessionsKey := userSessionsKey(userID)

	// Get count of user sessions
	count, err := s.client.ZCard(ctx, userSessionsKey).Result()
	if err != nil || count <= int64(s.maxSessionsPerUser) {
		return nil
	}

	// Remove oldest sessions
	toRemove := count - int64(s.maxSessionsPerUser)
	oldSessions, err := s.client.ZRange(ctx, userSessionsKey, 0, toRemove-1).Result()
	if err != nil {
		return nil
	}

	evicted := make([]string, 0, len(oldSessions))
	for _, sessionID := range oldSessions {
		sessionKey := sessionKey(sessionID)
		s.client.Del(ctx, sessionKey)
		s.client.ZRem(ctx, userSessionsKey, sessionID)
		evicted = append(evicted, sessionID)
	}

	return evicted
}

// UpdateSessionName updates the custom name of a session
func (s *SessionStore) UpdateSessionName(ctx context.Context, sessionID string, customName string) error {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	session.CustomName = customName

	sessionKey := sessionKey(sessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session expired")
	}

	err = s.client.Set(ctx, sessionKey, sessionData, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// sessionKey generates Redis key for a session
func sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// userSessionsKey generates Redis key for user's sessions list
func userSessionsKey(userID uint) string {
	return fmt.Sprintf("user:%d:sessions", userID)
}

// generateSessionID generates a cryptographically secure session ID
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
