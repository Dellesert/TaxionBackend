package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/user/usecase"
	sharedErrors "tachyon-messenger/shared/errors"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
)

// decodeBase64Flexible decodes base64 data trying multiple encodings:
// 1. Standard base64 with padding
// 2. URL-safe base64 without padding (used by WebAuthn/iOS)
// 3. URL-safe base64 with padding
// 4. Standard base64 without padding
func decodeBase64Flexible(data string) ([]byte, error) {
	// Try standard base64 with padding first
	if decoded, err := base64.StdEncoding.DecodeString(data); err == nil {
		return decoded, nil
	}

	// Try URL-safe base64 without padding (most common for WebAuthn)
	if decoded, err := base64.RawURLEncoding.DecodeString(data); err == nil {
		return decoded, nil
	}

	// Try URL-safe base64 with padding
	if decoded, err := base64.URLEncoding.DecodeString(data); err == nil {
		return decoded, nil
	}

	// Try standard base64 without padding
	if decoded, err := base64.RawStdEncoding.DecodeString(data); err == nil {
		return decoded, nil
	}

	return nil, fmt.Errorf("failed to decode base64 data with any encoding")
}

// PasskeyHandler handles passkey-related HTTP requests
type PasskeyHandler struct {
	passkeyUsecase usecase.PasskeyUsecase
}

// NewPasskeyHandler creates a new passkey handler
func NewPasskeyHandler(passkeyUsecase usecase.PasskeyUsecase) *PasskeyHandler {
	return &PasskeyHandler{
		passkeyUsecase: passkeyUsecase,
	}
}

// BeginRegistrationRequest represents the request to begin passkey registration
type BeginRegistrationRequest struct {
	Name string `json:"name"` // Optional - can be set later in FinishRegistration
}

// FinishRegistrationRequest represents the request to finish passkey registration
type FinishRegistrationRequest struct {
	Name string `json:"name" binding:"required"`
}

// BeginAuthenticationRequest represents the request to begin passkey authentication
type BeginAuthenticationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// UpdatePasskeyNameRequest represents the request to update a passkey's name
type UpdatePasskeyNameRequest struct {
	Name string `json:"name" binding:"required"`
}

// getOrigin extracts the origin from the request
func getOrigin(c *gin.Context) string {
	// Try Origin header first
	origin := c.GetHeader("Origin")
	if origin != "" {
		return origin
	}

	// Fallback to Referer
	referer := c.GetHeader("Referer")
	if referer != "" {
		// Extract origin from referer (protocol + host)
		if strings.HasPrefix(referer, "https://") {
			if hostEnd := findHostEnd(referer[8:]); hostEnd > 0 {
				return referer[:8+hostEnd]
			}
		}
		if strings.HasPrefix(referer, "http://") {
			if hostEnd := findHostEnd(referer[7:]); hostEnd > 0 {
				return referer[:7+hostEnd]
			}
		}
	}

	// Fallback to X-Forwarded-Host or Host header for iOS apps
	forwardedHost := c.GetHeader("X-Forwarded-Host")
	if forwardedHost != "" {
		// Assume HTTPS for production
		return "https://" + forwardedHost
	}

	host := c.GetHeader("Host")
	if host != "" && host != "localhost" && !strings.HasPrefix(host, "127.") {
		// Assume HTTPS for production
		return "https://" + host
	}

	return ""
}

// findHostEnd finds the end of host in a URL (first / or end of string)
func findHostEnd(s string) int {
	for i, c := range s {
		if c == '/' {
			return i
		}
	}
	return len(s)
}

// BeginRegistration starts the passkey registration process
// POST /api/v1/auth/passkey/register/begin
func (h *PasskeyHandler) BeginRegistration(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		apiErr := sharedErrors.UnauthorizedError("Unauthorized").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	var req BeginRegistrationRequest
	// Name is optional for begin registration
	_ = c.ShouldBindJSON(&req)

	// Use a default name if not provided
	name := req.Name
	if name == "" {
		name = "Passkey"
	}

	// Get origin from request
	origin := getOrigin(c)

	options, err := h.passkeyUsecase.BeginRegistration(userID.(uint), name, origin)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to begin passkey registration")
		apiErr := sharedErrors.NewAPIError(http.StatusBadRequest, sharedErrors.AuthPasskeyRegistrationFailed,
			"Failed to begin passkey registration").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	c.JSON(http.StatusOK, options)
}

// FinishRegistration completes the passkey registration process
// POST /api/v1/auth/passkey/register/finish
func (h *PasskeyHandler) FinishRegistration(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		apiErr := sharedErrors.UnauthorizedError("Unauthorized").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Read the raw body first
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to read request body")
		apiErr := sharedErrors.BadRequestError("Invalid request").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Parse the wrapper to get credential and name
	var wrapper struct {
		Credential json.RawMessage `json:"credential"`
		Name       string          `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to parse request wrapper")
		apiErr := sharedErrors.BadRequestError("Invalid request format").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Log the credential JSON for debugging
	logger.WithFields(map[string]interface{}{
		"name":            wrapper.Name,
		"credential_size": len(wrapper.Credential),
	}).Info("Received passkey registration data")
	logger.WithField("credential_json", string(wrapper.Credential)).Debug("Full credential data")

	// Parse credential from JSON with base64-encoded fields
	var credData struct {
		ID       string `json:"id"`
		RawID    string `json:"rawId"`
		Type     string `json:"type"`
		Response struct {
			ClientDataJSON    string `json:"clientDataJSON"`
			AttestationObject string `json:"attestationObject"`
		} `json:"response"`
	}
	if err := json.Unmarshal(wrapper.Credential, &credData); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to parse credential JSON")
		apiErr := sharedErrors.BadRequestError("Invalid credential format").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Decode base64 fields (supports both standard and URL-safe base64)
	rawID, err := decodeBase64Flexible(credData.RawID)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode rawId")
		apiErr := sharedErrors.BadRequestError("Invalid rawId encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	clientDataJSON, err := decodeBase64Flexible(credData.Response.ClientDataJSON)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode clientDataJSON")
		apiErr := sharedErrors.BadRequestError("Invalid clientDataJSON encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	attestationObject, err := decodeBase64Flexible(credData.Response.AttestationObject)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode attestationObject")
		apiErr := sharedErrors.BadRequestError("Invalid attestationObject encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Create the raw credential response structure
	credResponse := &protocol.CredentialCreationResponse{
		PublicKeyCredential: protocol.PublicKeyCredential{
			Credential: protocol.Credential{
				ID:   credData.ID,
				Type: credData.Type,
			},
			RawID: rawID,
		},
		AttestationResponse: protocol.AuthenticatorAttestationResponse{
			AuthenticatorResponse: protocol.AuthenticatorResponse{
				ClientDataJSON: clientDataJSON,
			},
			AttestationObject: attestationObject,
		},
	}

	// Parse the credential - this validates and fills the parsed fields
	parsedResponse, err := credResponse.Parse()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error":           err.Error(),
			"credential_id":   credData.ID,
			"credential_type": credData.Type,
		}).Error("Failed to parse credential response during registration")
		apiErr := sharedErrors.NewAPIError(http.StatusBadRequest, sharedErrors.AuthPasskeyInvalid,
			fmt.Sprintf("Invalid credential response: %v", err)).
			WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Get origin from request
	origin := getOrigin(c)

	// Finish registration
	if err := h.passkeyUsecase.FinishRegistration(userID.(uint), parsedResponse, origin); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to finish passkey registration")
		apiErr := sharedErrors.NewAPIError(http.StatusBadRequest, sharedErrors.AuthPasskeyRegistrationFailed,
			"Failed to finish passkey registration").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Passkey registered successfully",
		"request_id": requestID,
	})
}

// BeginAuthentication starts the passkey authentication process (legacy - requires email)
// POST /api/v1/auth/passkey/login/begin
func (h *PasskeyHandler) BeginAuthentication(c *gin.Context) {
	requestID := requestid.Get(c)

	var req BeginAuthenticationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := sharedErrors.BadRequestError("Invalid request").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	origin := getOrigin(c)
	options, err := h.passkeyUsecase.BeginAuthentication(req.Email, origin)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to begin passkey authentication")
		apiErr := sharedErrors.InvalidCredentialsError().WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	c.JSON(http.StatusOK, options)
}

// BeginDiscoverableAuthentication starts the passkey authentication process WITHOUT email
// POST /api/v1/auth/passkey/login/discoverable/begin
func (h *PasskeyHandler) BeginDiscoverableAuthentication(c *gin.Context) {
	requestID := requestid.Get(c)

	origin := getOrigin(c)
	logger.WithFields(map[string]interface{}{
		"origin":           origin,
		"origin_header":    c.GetHeader("Origin"),
		"referer":          c.GetHeader("Referer"),
		"host":             c.GetHeader("Host"),
		"x_forwarded_host": c.GetHeader("X-Forwarded-Host"),
		"request_id":       requestID,
	}).Info("BeginDiscoverableAuthentication called")

	options, err := h.passkeyUsecase.BeginDiscoverableAuthentication(origin)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to begin discoverable passkey authentication")
		apiErr := sharedErrors.NewAPIError(http.StatusBadRequest, sharedErrors.AuthPasskeyInvalid,
			"Passkey authentication failed").
			WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	c.JSON(http.StatusOK, options)
}

// FinishAuthentication completes the passkey authentication process
// POST /api/v1/auth/passkey/login/finish
func (h *PasskeyHandler) FinishAuthentication(c *gin.Context) {
	requestID := requestid.Get(c)

	// Read the raw body first
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to read request body")
		apiErr := sharedErrors.BadRequestError("Invalid request").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Parse credential from JSON with base64-encoded fields
	var credData struct {
		ID       string `json:"id"`
		RawID    string `json:"rawId"`
		Type     string `json:"type"`
		Response struct {
			ClientDataJSON    string  `json:"clientDataJSON"`
			AuthenticatorData string  `json:"authenticatorData"`
			Signature         string  `json:"signature"`
			UserHandle        *string `json:"userHandle"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bodyBytes, &credData); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to parse credential JSON")
		apiErr := sharedErrors.BadRequestError("Invalid credential format").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Decode base64 fields (supports both standard and URL-safe base64)
	rawID, err := decodeBase64Flexible(credData.RawID)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode rawId")
		apiErr := sharedErrors.BadRequestError("Invalid rawId encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	clientDataJSON, err := decodeBase64Flexible(credData.Response.ClientDataJSON)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode clientDataJSON")
		apiErr := sharedErrors.BadRequestError("Invalid clientDataJSON encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	authenticatorData, err := decodeBase64Flexible(credData.Response.AuthenticatorData)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode authenticatorData")
		apiErr := sharedErrors.BadRequestError("Invalid authenticatorData encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	signature, err := decodeBase64Flexible(credData.Response.Signature)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode signature")
		apiErr := sharedErrors.BadRequestError("Invalid signature encoding").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	var userHandle []byte
	if credData.Response.UserHandle != nil && *credData.Response.UserHandle != "" {
		userHandle, err = decodeBase64Flexible(*credData.Response.UserHandle)
		if err != nil {
			logger.WithField("error", err.Error()).Warn("Failed to decode userHandle")
			apiErr := sharedErrors.BadRequestError("Invalid userHandle encoding").WithRequestID(requestID)
			c.JSON(apiErr.StatusCode, apiErr)
			return
		}
	}

	// Create the credential request response structure
	credResponse := &protocol.CredentialAssertionResponse{
		PublicKeyCredential: protocol.PublicKeyCredential{
			Credential: protocol.Credential{
				ID:   credData.ID,
				Type: credData.Type,
			},
			RawID: rawID,
		},
		AssertionResponse: protocol.AuthenticatorAssertionResponse{
			AuthenticatorResponse: protocol.AuthenticatorResponse{
				ClientDataJSON: clientDataJSON,
			},
			AuthenticatorData: authenticatorData,
			Signature:         signature,
			UserHandle:        userHandle,
		},
	}

	// Parse the credential - this validates and fills the parsed fields
	parsedResponse, err := credResponse.Parse()
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to parse credential response")
		apiErr := sharedErrors.NewAPIError(http.StatusBadRequest, sharedErrors.AuthPasskeyInvalid,
			"Invalid credential response").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Get origin from request
	origin := getOrigin(c)

	// Get the credential ID to look up the user
	// We need to find which user this credential belongs to
	user, err := h.passkeyUsecase.FinishAuthenticationByCredential(parsedResponse, origin)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to finish passkey authentication")
		apiErr := sharedErrors.NewAPIError(http.StatusUnauthorized, sharedErrors.AuthPasskeyInvalid,
			"Invalid passkey").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Create session for the user
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	authConfig := middleware.GetAuthConfig()
	if authConfig != nil && authConfig.SessionStore != nil {
		ctx := c.Request.Context()
		session, err := authConfig.SessionStore.CreateSession(
			ctx,
			user.ID,
			user.Email,
			user.Role,
			ipAddress,
			userAgent,
		)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    user.ID,
				"error":      err.Error(),
			}).Error("Failed to create session after passkey authentication")

			apiErr := sharedErrors.InternalError("Failed to create session").
				WithRequestID(requestID)
			c.JSON(apiErr.StatusCode, apiErr)
			return
		}

		// Set session cookie
		c.SetCookie(
			"session_id",
			session.SessionID,
			int(session.ExpiresAt.Unix()-session.CreatedAt.Unix()),
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			true,  // httpOnly
		)

		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    user.ID,
			"session_id": session.SessionID,
		}).Info("Passkey authentication successful, session created")

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Authentication successful",
			"user":    user.ToResponse(),
			"session": gin.H{
				"session_id": session.SessionID,
			},
			"auth_mode":  "session",
			"request_id": requestID,
		})
	} else {
		logger.WithField("user_id", user.ID).Error("Session store not configured")
		apiErr := sharedErrors.InternalError("Session management not available").
			WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
	}
}

// ListPasskeys lists all passkeys for the authenticated user
// GET /api/v1/auth/passkey
func (h *PasskeyHandler) ListPasskeys(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	passkeys, err := h.passkeyUsecase.ListUserPasskeys(userID.(uint))
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to list passkeys")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get passkeys"})
		return
	}

	// Convert to response format (hide sensitive data)
	response := make([]gin.H, len(passkeys))
	for i, pk := range passkeys {
		response[i] = gin.H{
			"id":         pk.ID,
			"name":       pk.Name,
			"created_at": pk.CreatedAt,
			"last_used":  pk.LastUsedAt,
			"transports": pk.Transports,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"passkeys": response,
	})
}

// DeletePasskey deletes a passkey
// DELETE /api/v1/auth/passkey/:id
func (h *PasskeyHandler) DeletePasskey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	passkeyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}

	if err := h.passkeyUsecase.DeletePasskey(userID.(uint), uint(passkeyID)); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to delete passkey")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey deleted successfully",
	})
}

// UpdatePasskeyName updates a passkey's name
// PATCH /api/v1/auth/passkey/:id
func (h *PasskeyHandler) UpdatePasskeyName(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	passkeyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Invalid passkey ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}

	// Read raw body for debugging
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":    userID,
		"passkey_id": passkeyID,
		"body":       string(bodyBytes),
	}).Info("Updating passkey name")

	// Parse the request
	var req UpdatePasskeyNameRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to parse update name request")
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request format: %v", err)})
		return
	}

	logger.WithField("new_name", req.Name).Info("Parsed passkey name from request")

	if err := h.passkeyUsecase.UpdatePasskeyName(userID.(uint), uint(passkeyID), req.Name); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to update passkey name")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey name updated successfully",
	})
}
