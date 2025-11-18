// File: services/gateway/proxy.go
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ServiceConfig holds configuration for a microservice
type ServiceConfig struct {
	Name string
	URL  string
}

// ProxyConfig holds all service configurations
type ProxyConfig struct {
	UserService         ServiceConfig
	ChatService         ServiceConfig
	TaskService         ServiceConfig
	CalendarService     ServiceConfig
	PollService         ServiceConfig
	NotificationService ServiceConfig
	FileService         ServiceConfig
	AnalyticsService    ServiceConfig
	BackupService       ServiceConfig
}

// getProxyConfig returns service URLs configuration
func getProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		UserService: ServiceConfig{
			Name: "user-service",
			URL:  getEnvOrDefault("USER_SERVICE_URL", "http://localhost:8081"),
		},
		ChatService: ServiceConfig{
			Name: "chat-service",
			URL:  getEnvOrDefault("CHAT_SERVICE_URL", "http://localhost:8082"),
		},
		TaskService: ServiceConfig{
			Name: "task-service",
			URL:  getEnvOrDefault("TASK_SERVICE_URL", "http://localhost:8083"),
		},
		CalendarService: ServiceConfig{
			Name: "calendar-service",
			URL:  getEnvOrDefault("CALENDAR_SERVICE_URL", "http://localhost:8084"),
		},
		PollService: ServiceConfig{
			Name: "poll-service",
			URL:  getEnvOrDefault("POLL_SERVICE_URL", "http://localhost:8085"),
		},
		NotificationService: ServiceConfig{
			Name: "notification-service",
			URL:  getEnvOrDefault("NOTIFICATION_SERVICE_URL", "http://localhost:8087"),
		},
		FileService: ServiceConfig{
			Name: "file-service",
			URL:  getEnvOrDefault("FILE_SERVICE_URL", "http://localhost:8088"),
		},
		AnalyticsService: ServiceConfig{
			Name: "analytics-service",
			URL:  getEnvOrDefault("ANALYTICS_SERVICE_URL", "http://localhost:8086"),
		},
		BackupService: ServiceConfig{
			Name: "backup-service",
			URL:  getEnvOrDefault("BACKUP_SERVICE_URL", "http://localhost:8089"),
		},
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development, adjust for production
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// proxyRequest handles proxying HTTP requests to microservices
func proxyRequest(targetURL, serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)
		startTime := time.Now()

		// Check if this is a WebSocket upgrade request
		if isWebSocketRequest(c.Request) {
			proxyWebSocket(c, targetURL, serviceName, requestID)
			return
		}

		// Parse target URL
		target, err := url.Parse(targetURL)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"error":      err.Error(),
				"target_url": targetURL,
			}).Error("Failed to parse target URL")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Service configuration error",
				"request_id": requestID,
			})
			return
		}

		// Build proxy URL
		proxyURL := &url.URL{
			Scheme:   target.Scheme,
			Host:     target.Host,
			Path:     c.Request.URL.Path,
			RawQuery: c.Request.URL.RawQuery,
		}

		// For multipart/form-data, we need to forward the body directly without reading it
		contentType := c.GetHeader("Content-Type")
		isMultipart := strings.HasPrefix(contentType, "multipart/form-data")

		var bodyReader io.Reader
		if isMultipart {
			// For multipart, forward the body directly
			bodyReader = c.Request.Body
		} else {
			// For other content types, read and buffer the body
			var bodyBytes []byte
			if c.Request.Body != nil {
				bodyBytes, err = io.ReadAll(c.Request.Body)
				if err != nil {
					logger.WithFields(map[string]interface{}{
						"request_id": requestID,
						"service":    serviceName,
						"error":      err.Error(),
					}).Error("Failed to read request body")

					c.JSON(http.StatusBadRequest, gin.H{
						"error":      "Failed to read request body",
						"request_id": requestID,
					})
					return
				}
			}
			bodyReader = bytes.NewReader(bodyBytes)
		}

		// Create proxy request
		proxyReq, err := http.NewRequest(c.Request.Method, proxyURL.String(), bodyReader)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"error":      err.Error(),
				"proxy_url":  proxyURL.String(),
			}).Error("Failed to create proxy request")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to create proxy request",
				"request_id": requestID,
			})
			return
		}

		// Copy headers from original request
		copyHeaders(c.Request.Header, proxyReq.Header)

		// Add request ID to forwarded request
		proxyReq.Header.Set("X-Request-ID", requestID)
		proxyReq.Header.Set("X-Forwarded-For", c.ClientIP())
		proxyReq.Header.Set("X-Forwarded-Proto", c.Request.Header.Get("X-Forwarded-Proto"))

		// Debug: Log User-Agent headers
		logger.WithFields(map[string]interface{}{
			"request_id":        requestID,
			"service":           serviceName,
			"x_device_info_in":  c.Request.Header.Get("X-Device-Info"),
			"x_user_agent_in":   c.Request.Header.Get("X-User-Agent"),
			"user_agent_in":     c.Request.Header.Get("User-Agent"),
			"x_device_info_out": proxyReq.Header.Get("X-Device-Info"),
			"x_user_agent_out":  proxyReq.Header.Get("X-User-Agent"),
			"user_agent_out":    proxyReq.Header.Get("User-Agent"),
			"path":              c.Request.URL.Path,
		}).Info("Gateway headers forwarding")

		// Log proxy request
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"service":    serviceName,
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"proxy_url":  proxyURL.String(),
			"client_ip":  c.ClientIP(),
		}).Info("Proxying request to service")

		// Make the request
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		resp, err := client.Do(proxyReq)
		if err != nil {
			duration := time.Since(startTime)
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"error":      err.Error(),
				"duration":   duration,
			}).Error("Proxy request failed")

			c.JSON(http.StatusBadGateway, gin.H{
				"error":      fmt.Sprintf("Service %s is unavailable", serviceName),
				"request_id": requestID,
			})
			return
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			duration := time.Since(startTime)
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"error":      err.Error(),
				"duration":   duration,
			}).Error("Failed to read response body")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to read service response",
				"request_id": requestID,
			})
			return
		}

		// Copy response headers
		copyHeaders(resp.Header, c.Writer.Header())

		// Log successful proxy response
		duration := time.Since(startTime)
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"service":       serviceName,
			"status_code":   resp.StatusCode,
			"duration":      duration,
			"response_size": len(respBody),
		}).Info("Proxy request completed")

		// Send response
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

// copyHeaders copies headers from source to destination
func copyHeaders(src, dst http.Header) {
	for key, values := range src {
		// Skip headers that should not be forwarded
		if shouldSkipHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// shouldSkipHeader determines if a header should be skipped during forwarding
func shouldSkipHeader(header string) bool {
	// Headers that should not be forwarded (for HTTP requests, not WebSocket)
	skipHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
		// Skip CORS headers from backend services - gateway handles CORS
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Credentials",
		"Access-Control-Expose-Headers",
		"Access-Control-Max-Age",
	}

	headerLower := strings.ToLower(header)
	for _, skip := range skipHeaders {
		if headerLower == strings.ToLower(skip) {
			return true
		}
	}
	return false
}

// isWebSocketRequest checks if the request is a WebSocket upgrade request
func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Connection")) == "upgrade" &&
		strings.ToLower(r.Header.Get("Upgrade")) == "websocket"
}

// proxyWebSocket handles WebSocket connection proxying
func proxyWebSocket(c *gin.Context, targetURL, serviceName, requestID string) {
	// Parse target URL and convert to WebSocket URL
	target, err := url.Parse(targetURL)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"service":    serviceName,
			"error":      err.Error(),
		}).Error("Failed to parse target URL for WebSocket")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service configuration error"})
		return
	}

	// Build WebSocket URL
	wsScheme := "ws"
	if target.Scheme == "https" {
		wsScheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s%s", wsScheme, target.Host, c.Request.URL.Path)
	if c.Request.URL.RawQuery != "" {
		wsURL += "?" + c.Request.URL.RawQuery
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"service":    serviceName,
		"ws_url":     wsURL,
		"client_ip":  c.ClientIP(),
	}).Info("Proxying WebSocket connection")

	// Prepare headers for backend WebSocket connection
	backendHeaders := http.Header{}
	for key, values := range c.Request.Header {
		// Copy relevant headers
		if !shouldSkipWebSocketHeader(key) {
			for _, value := range values {
				backendHeaders.Add(key, value)
			}
		}
	}

	// Add forwarding headers
	backendHeaders.Set("X-Request-ID", requestID)
	backendHeaders.Set("X-Forwarded-For", c.ClientIP())
	backendHeaders.Set("X-Real-IP", c.ClientIP())

	// Connect to backend WebSocket
	backendConn, backendResp, err := websocket.DefaultDialer.Dial(wsURL, backendHeaders)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"service":    serviceName,
			"error":      err.Error(),
			"ws_url":     wsURL,
		}).Error("Failed to connect to backend WebSocket")

		statusCode := http.StatusBadGateway
		if backendResp != nil {
			statusCode = backendResp.StatusCode
		}
		c.JSON(statusCode, gin.H{"error": fmt.Sprintf("Failed to connect to %s WebSocket", serviceName)})
		return
	}
	defer backendConn.Close()

	// Upgrade client connection to WebSocket
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"service":    serviceName,
			"error":      err.Error(),
		}).Error("Failed to upgrade client connection to WebSocket")
		return
	}
	defer clientConn.Close()

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"service":    serviceName,
	}).Info("WebSocket proxy connection established")

	// Proxy messages bidirectionally
	errChan := make(chan error, 2)

	// Client -> Backend
	go func() {
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("client read error: %w", err)
				return
			}

			if err := backendConn.WriteMessage(messageType, message); err != nil {
				errChan <- fmt.Errorf("backend write error: %w", err)
				return
			}
		}
	}()

	// Backend -> Client
	go func() {
		for {
			messageType, message, err := backendConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("backend read error: %w", err)
				return
			}

			if err := clientConn.WriteMessage(messageType, message); err != nil {
				errChan <- fmt.Errorf("client write error: %w", err)
				return
			}
		}
	}()

	// Wait for error or connection close
	err = <-errChan
	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"service":    serviceName,
		"error":      err.Error(),
	}).Info("WebSocket proxy connection closed")
}

// shouldSkipWebSocketHeader determines if a header should be skipped for WebSocket proxying
func shouldSkipWebSocketHeader(header string) bool {
	// For WebSocket, we want to preserve most headers but skip connection-specific ones
	// that websocket.Dialer will set automatically
	skipHeaders := []string{
		"Connection",
		"Upgrade",
		"Sec-Websocket-Key",
		"Sec-Websocket-Version",
		"Sec-Websocket-Extensions",
		"Sec-Websocket-Accept",
		// Skip CORS headers - gateway handles CORS
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Credentials",
	}

	headerLower := strings.ToLower(header)
	for _, skip := range skipHeaders {
		if headerLower == strings.ToLower(skip) {
			return true
		}
	}
	return false
}
