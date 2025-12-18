package usecase

import (
	"fmt"
	"os"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/logger"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnService wraps the WebAuthn library with support for multiple origins
type WebAuthnService struct {
	rpDisplayName string
	// Map of origin -> WebAuthn instance
	webAuthnInstances map[string]*webauthn.WebAuthn
	// Map of origin -> RP_ID
	originToRPID map[string]string
	// All configured origins
	allOrigins []string
}

// NewWebAuthnService creates a new WebAuthn service with multi-origin support
func NewWebAuthnService() (*WebAuthnService, error) {
	rpDisplayName := os.Getenv("WEBAUTHN_RP_DISPLAY_NAME")
	if rpDisplayName == "" {
		rpDisplayName = "Tachyon Messenger"
	}

	rpOrigin := os.Getenv("WEBAUTHN_RP_ORIGIN")
	if rpOrigin == "" {
		rpOrigin = "http://localhost:8081" // Default for development
	}

	// Parse comma-separated origins
	origins := strings.Split(rpOrigin, ",")
	allOrigins := make([]string, 0, len(origins))
	for _, origin := range origins {
		allOrigins = append(allOrigins, strings.TrimSpace(origin))
	}

	// Create origin -> RP_ID mapping based on domain extraction
	originToRPID := make(map[string]string)
	for _, origin := range allOrigins {
		rpID := extractRPIDFromOrigin(origin)
		originToRPID[origin] = rpID
		logger.WithFields(map[string]interface{}{
			"origin": origin,
			"rp_id":  rpID,
		}).Info("Configured WebAuthn RP_ID mapping")
	}

	// Pre-create WebAuthn instances for each unique RP_ID
	webAuthnInstances := make(map[string]*webauthn.WebAuthn)
	rpIDToOrigins := make(map[string][]string)

	// Group origins by RP_ID
	for origin, rpID := range originToRPID {
		rpIDToOrigins[rpID] = append(rpIDToOrigins[rpID], origin)
	}

	// Create one WebAuthn instance per RP_ID
	for rpID, origins := range rpIDToOrigins {
		wconfig := &webauthn.Config{
			RPDisplayName:         rpDisplayName,
			RPID:                  rpID,
			RPOrigins:             origins,
			AttestationPreference: protocol.PreferNoAttestation,
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				RequireResidentKey: protocol.ResidentKeyNotRequired(),
				ResidentKey:        protocol.ResidentKeyRequirementPreferred,
				UserVerification:   protocol.VerificationPreferred,
			},
		}

		web, err := webauthn.New(wconfig)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"rp_id":   rpID,
				"origins": origins,
				"error":   err.Error(),
			}).Error("Failed to create WebAuthn instance")
			return nil, fmt.Errorf("failed to create WebAuthn instance for RP_ID %s: %w", rpID, err)
		}

		// Store this instance for all origins that use this RP_ID
		for _, origin := range origins {
			webAuthnInstances[origin] = web
		}

		logger.WithFields(map[string]interface{}{
			"rp_id":   rpID,
			"origins": origins,
		}).Info("Created WebAuthn instance")
	}

	return &WebAuthnService{
		rpDisplayName:     rpDisplayName,
		webAuthnInstances: webAuthnInstances,
		originToRPID:      originToRPID,
		allOrigins:        allOrigins,
	}, nil
}

// extractRPIDFromOrigin extracts the RP_ID (domain) from an origin URL
// For https://dash.fusioninsight.cloud -> dash.fusioninsight.cloud
// For http://localhost:5173 -> localhost
func extractRPIDFromOrigin(origin string) string {
	// Remove protocol
	origin = strings.TrimPrefix(origin, "https://")
	origin = strings.TrimPrefix(origin, "http://")

	// Remove port if present
	if idx := strings.Index(origin, ":"); idx != -1 {
		origin = origin[:idx]
	}

	// Remove path if present
	if idx := strings.Index(origin, "/"); idx != -1 {
		origin = origin[:idx]
	}

	return origin
}

// GetWebAuthnForOrigin returns the appropriate WebAuthn instance for the given origin
func (s *WebAuthnService) GetWebAuthnForOrigin(origin string) (*webauthn.WebAuthn, error) {
	// Normalize origin
	origin = strings.TrimSpace(origin)

	// Try exact match first
	if instance, ok := s.webAuthnInstances[origin]; ok {
		return instance, nil
	}

	// If no exact match, try to find by RP_ID
	rpID := extractRPIDFromOrigin(origin)
	for configuredOrigin, configuredRPID := range s.originToRPID {
		if configuredRPID == rpID {
			if instance, ok := s.webAuthnInstances[configuredOrigin]; ok {
				logger.WithFields(map[string]interface{}{
					"requested_origin":   origin,
					"matched_origin":     configuredOrigin,
					"rp_id":              rpID,
				}).Debug("Matched WebAuthn instance by RP_ID")
				return instance, nil
			}
		}
	}

	// Fallback: use the first available instance (for backward compatibility)
	if len(s.webAuthnInstances) > 0 {
		for _, instance := range s.webAuthnInstances {
			logger.WithField("origin", origin).Warn("No matching WebAuthn instance found, using fallback")
			return instance, nil
		}
	}

	return nil, fmt.Errorf("no WebAuthn instance configured for origin: %s", origin)
}

// WebAuthnUser implements the webauthn.User interface
type WebAuthnUser struct {
	user        *models.User
	credentials []webauthn.Credential
}

// NewWebAuthnUser creates a new WebAuthnUser from a models.User
func NewWebAuthnUser(user *models.User, passkeyCredentials []*models.PasskeyCredential) *WebAuthnUser {
	credentials := make([]webauthn.Credential, 0, len(passkeyCredentials))

	for _, pk := range passkeyCredentials {
		transports := parseTransports(pk.Transports)
		credentials = append(credentials, webauthn.Credential{
			ID:              pk.CredentialID,
			PublicKey:       pk.PublicKey,
			AttestationType: pk.AttestationType,
			Transport:       transports,
			Flags: webauthn.CredentialFlags{
				UserPresent:    true,
				UserVerified:   true,
				BackupEligible: pk.BackupEligible,
				BackupState:    pk.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    pk.AAGUID,
				SignCount: pk.SignCount,
			},
		})
	}

	return &WebAuthnUser{
		user:        user,
		credentials: credentials,
	}
}

// WebAuthnID returns the user's ID
func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.user.ID))
}

// WebAuthnName returns the user's email
func (u *WebAuthnUser) WebAuthnName() string {
	return u.user.Email
}

// WebAuthnDisplayName returns the user's display name
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.user.Name
}

// WebAuthnIcon returns the user's icon URL (empty for now)
func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

// WebAuthnCredentials returns the user's credentials
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

// parseTransports converts a comma-separated string of transports to a slice
func parseTransports(transportsStr string) []protocol.AuthenticatorTransport {
	if transportsStr == "" {
		return []protocol.AuthenticatorTransport{}
	}

	// Simple parsing - in production you might want more robust parsing
	transports := []protocol.AuthenticatorTransport{}

	// Common transports
	if containsTransport(transportsStr, "usb") {
		transports = append(transports, protocol.USB)
	}
	if containsTransport(transportsStr, "nfc") {
		transports = append(transports, protocol.NFC)
	}
	if containsTransport(transportsStr, "ble") {
		transports = append(transports, protocol.BLE)
	}
	if containsTransport(transportsStr, "internal") {
		transports = append(transports, protocol.Internal)
	}

	return transports
}

// containsTransport checks if a transport string contains a specific transport
func containsTransport(haystack, needle string) bool {
	// Simple substring check - in production use proper parsing
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle ||
			haystack[:len(needle)] == needle ||
			haystack[len(haystack)-len(needle):] == needle)
}
