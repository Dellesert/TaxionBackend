package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// NotifyEvictedSessions sends async WebSocket disconnect requests for each evicted session.
// The reason is included so the client can display the appropriate message.
func NotifyEvictedSessions(evictedSessionIDs []string, reason string) {
	if len(evictedSessionIDs) == 0 {
		return
	}

	for _, sessionID := range evictedSessionIDs {
		go notifySessionDisconnect(sessionID, reason)
	}
}

// notifySessionDisconnect sends a request to chat-service to disconnect a specific session.
func notifySessionDisconnect(sessionID, reason string) {
	chatServiceURL := os.Getenv("CHAT_SERVICE_URL")
	if chatServiceURL == "" {
		chatServiceURL = "http://chat-service:8082"
	}

	payload, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"reason":     reason,
	})

	url := fmt.Sprintf("%s/api/v1/internal/ws/disconnect-session", chatServiceURL)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Failed to create disconnect request for session %s: %v", sessionID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to disconnect evicted session %s: %v", sessionID, err)
		return
	}
	defer resp.Body.Close()
}
