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
)

// ServiceConfig holds configuration for a microservice
type ServiceConfig struct {
	Name string
	URL  string
}

// ProxyConfig holds all service configurations
type ProxyConfig struct {
	UserService ServiceConfig
	// Add other services here as needed
}

// getProxyConfig returns service URLs configuration
func getProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		UserService: ServiceConfig{
			Name: "user-service",
			URL:  getEnvOrDefault("USER_SERVICE_URL", "http://localhost:8081"),
		},
		// Add other services here
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// proxyRequest handles proxying HTTP requests to microservices
func proxyRequest(targetURL, serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)
		startTime := time.Now()

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

		// Read request body
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

		// Create proxy request
		proxyReq, err := http.NewRequest(c.Request.Method, proxyURL.String(), bytes.NewReader(bodyBytes))
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
	// Headers that should not be forwarded
	skipHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	headerLower := strings.ToLower(header)
	for _, skip := range skipHeaders {
		if headerLower == strings.ToLower(skip) {
			return true
		}
	}
	return false
}
