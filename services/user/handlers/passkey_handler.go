package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
)

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

// BeginRegistration starts the passkey registration process
// POST /api/v1/auth/passkey/register/begin
func (h *PasskeyHandler) BeginRegistration(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	options, err := h.passkeyUsecase.BeginRegistration(userID.(uint), name)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to begin passkey registration")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, options)
}

// FinishRegistration completes the passkey registration process
// POST /api/v1/auth/passkey/register/finish
func (h *PasskeyHandler) FinishRegistration(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Read the raw body first
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Parse the wrapper to get credential and name
	var wrapper struct {
		Credential json.RawMessage `json:"credential"`
		Name       string          `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to parse request wrapper")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	// Log the credential JSON for debugging
	logger.WithFields(map[string]interface{}{
		"name": wrapper.Name,
	}).Info("Received passkey registration data")

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential format"})
		return
	}

	// Decode base64 fields (using standard base64 decoding)
	rawID, err := base64.StdEncoding.DecodeString(credData.RawID)
	if err != nil {
		// Try URL-safe base64
		rawID, err = base64.RawURLEncoding.DecodeString(credData.RawID)
		if err != nil {
			logger.WithField("error", err.Error()).Warn("Failed to decode rawId")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rawId encoding"})
			return
		}
	}

	clientDataJSON, err := base64.StdEncoding.DecodeString(credData.Response.ClientDataJSON)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode clientDataJSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clientDataJSON encoding"})
		return
	}

	attestationObject, err := base64.StdEncoding.DecodeString(credData.Response.AttestationObject)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode attestationObject")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid attestationObject encoding"})
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
		logger.WithField("error", err.Error()).Warn("Failed to parse credential response")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential response"})
		return
	}

	// Finish registration
	if err := h.passkeyUsecase.FinishRegistration(userID.(uint), parsedResponse); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to finish passkey registration")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Passkey registered successfully",
	})
}

// BeginAuthentication starts the passkey authentication process
// POST /api/v1/auth/passkey/login/begin
func (h *PasskeyHandler) BeginAuthentication(c *gin.Context) {
	var req BeginAuthenticationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	options, err := h.passkeyUsecase.BeginAuthentication(req.Email)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to begin passkey authentication")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, options)
}

// FinishAuthentication completes the passkey authentication process
// POST /api/v1/auth/passkey/login/finish
func (h *PasskeyHandler) FinishAuthentication(c *gin.Context) {
	// Read the raw body first
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential format"})
		return
	}

	// Decode base64 fields
	rawID, err := base64.StdEncoding.DecodeString(credData.RawID)
	if err != nil {
		// Try URL-safe base64
		rawID, err = base64.RawURLEncoding.DecodeString(credData.RawID)
		if err != nil {
			logger.WithField("error", err.Error()).Warn("Failed to decode rawId")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rawId encoding"})
			return
		}
	}

	clientDataJSON, err := base64.StdEncoding.DecodeString(credData.Response.ClientDataJSON)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode clientDataJSON")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid clientDataJSON encoding"})
		return
	}

	authenticatorData, err := base64.StdEncoding.DecodeString(credData.Response.AuthenticatorData)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode authenticatorData")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authenticatorData encoding"})
		return
	}

	signature, err := base64.StdEncoding.DecodeString(credData.Response.Signature)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to decode signature")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature encoding"})
		return
	}

	var userHandle []byte
	if credData.Response.UserHandle != nil && *credData.Response.UserHandle != "" {
		userHandle, err = base64.StdEncoding.DecodeString(*credData.Response.UserHandle)
		if err != nil {
			logger.WithField("error", err.Error()).Warn("Failed to decode userHandle")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userHandle encoding"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential response"})
		return
	}

	// Get the credential ID to look up the user
	// We need to find which user this credential belongs to
	user, err := h.passkeyUsecase.FinishAuthenticationByCredential(parsedResponse)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to finish passkey authentication")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid passkey"})
		return
	}

	// Create session for the user
	requestID := c.GetHeader("X-Request-ID")
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

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to create session",
				"request_id": requestID,
			})
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
			"auth_mode": "session",
		})
	} else {
		logger.WithField("user_id", user.ID).Error("Session store not configured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Session management not available"})
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
			"id":          pk.ID,
			"name":        pk.Name,
			"created_at":  pk.CreatedAt,
			"last_used":   pk.LastUsedAt,
			"transports":  pk.Transports,
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}

	var req UpdatePasskeyNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
