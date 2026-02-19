package usecase

import (
	"encoding/hex"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/logger"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// RPConfig holds configuration for a single Relying Party
type RPConfig struct {
	RPID      string
	RPOrigins []string
}

// WebAuthnService wraps the WebAuthn library with multi-RP support
type WebAuthnService struct {
	webAuthn      *webauthn.WebAuthn // Default/primary WebAuthn instance
	rpConfigs     map[string]*webauthn.WebAuthn // Map of RP ID -> WebAuthn instance
	originToRPID  map[string]string // Map of origin -> RP ID
	rpDisplayName string
}

// NewWebAuthnService creates a new WebAuthn service with multi-RP support
func NewWebAuthnService() (*WebAuthnService, error) {
	rpDisplayName := os.Getenv("WEBAUTHN_RP_DISPLAY_NAME")
	if rpDisplayName == "" {
		rpDisplayName = "Tachyon Messenger"
	}

	// Parse RP IDs (comma-separated) - OPTIONAL
	// If not specified, RP ID will be extracted from origin hostname
	rpIDsStr := os.Getenv("WEBAUTHN_RP_ID")
	var rpIDs []string
	if rpIDsStr != "" {
		rpIDs = parseCommaSeparated(rpIDsStr)
	}

	// Parse origins (comma-separated)
	rpOriginsStr := os.Getenv("WEBAUTHN_RP_ORIGIN")
	if rpOriginsStr == "" {
		rpOriginsStr = "http://localhost:8081"
	}
	rpOrigins := parseCommaSeparated(rpOriginsStr)

	// Add Android origins if SHA256 fingerprints are configured
	// Supports multiple fingerprints separated by commas (e.g., for dev and release builds)
	androidFingerprints := os.Getenv("ANDROID_SHA256_FINGERPRINT")
	if androidFingerprints != "" {
		for _, fp := range parseCommaSeparated(androidFingerprints) {
			androidOrigin := generateAndroidOrigin(fp)
			if androidOrigin != "" {
				rpOrigins = append(rpOrigins, androidOrigin)
				logger.WithField("android_origin", androidOrigin).Info("Added Android app origin for Passkeys")
			}
		}
	}

	// Build origin to RP ID mapping
	originToRPID := make(map[string]string)
	rpConfigs := make(map[string]*webauthn.WebAuthn)

	// Group origins by their RP ID
	rpIDToOrigins := make(map[string][]string)

	// Get primary RP ID for Android origins (from first HTTPS origin)
	var primaryRPID string
	for _, origin := range rpOrigins {
		if strings.HasPrefix(origin, "https://") {
			primaryRPID = extractHostname(origin)
			break
		}
	}

	for _, origin := range rpOrigins {
		var rpID string

		// Handle Android origins specially - they use the same RP ID as the web domain
		if strings.HasPrefix(origin, "android:") {
			// Android origins use the primary RP ID (the domain from assetlinks.json)
			if primaryRPID != "" {
				rpID = primaryRPID
			} else {
				logger.Warn("Android origin configured but no HTTPS origin found for RP ID")
				continue
			}
		} else if strings.HasPrefix(origin, "app://") || strings.HasPrefix(origin, "electron://") {
			// Electron app origins (app://local, electron://) use the primary RP ID
			// This enables Passkey support in Electron desktop apps via WebAuthn Related Origins
			if primaryRPID != "" {
				rpID = primaryRPID
			} else {
				logger.Warn("Electron origin configured but no HTTPS origin found for RP ID")
				continue
			}
		} else if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			// Localhost origins (Electron dev mode) use the primary RP ID
			// so passkeys are tied to the production domain, not "localhost"
			if primaryRPID != "" {
				rpID = primaryRPID
			} else {
				// Fallback: use localhost as RP ID if no HTTPS origin exists
				hostname := extractHostname(origin)
				rpID = hostname
			}
		} else {
			// Extract hostname from origin (e.g., "https://taxion.fusioninsight.cloud" -> "taxion.fusioninsight.cloud")
			hostname := extractHostname(origin)

			// If RP IDs are specified, find matching one; otherwise use hostname as RP ID
			if len(rpIDs) > 0 {
				rpID = findMatchingRPID(hostname, rpIDs)
			} else {
				// Auto-detect: use hostname as RP ID
				rpID = hostname
			}
		}

		originToRPID[origin] = rpID
		rpIDToOrigins[rpID] = append(rpIDToOrigins[rpID], origin)

		logger.WithFields(map[string]interface{}{
			"origin": origin,
			"rp_id":  rpID,
		}).Info("Mapped origin to RP ID")
	}

	// Create WebAuthn instance for each unique RP ID
	var primaryWebAuthn *webauthn.WebAuthn
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
				"error": err.Error(),
				"rp_id": rpID,
			}).Error("Failed to create WebAuthn instance")
			return nil, fmt.Errorf("failed to create WebAuthn instance for RP ID %s: %w", rpID, err)
		}

		rpConfigs[rpID] = web
		if primaryWebAuthn == nil {
			primaryWebAuthn = web
		}

		logger.WithFields(map[string]interface{}{
			"rp_id":   rpID,
			"origins": origins,
		}).Info("Created WebAuthn instance for RP")
	}

	if primaryWebAuthn == nil {
		return nil, fmt.Errorf("no WebAuthn instances created")
	}

	return &WebAuthnService{
		webAuthn:      primaryWebAuthn,
		rpConfigs:     rpConfigs,
		originToRPID:  originToRPID,
		rpDisplayName: rpDisplayName,
	}, nil
}

// GetWebAuthnForOrigin returns the WebAuthn instance for a given origin
func (s *WebAuthnService) GetWebAuthnForOrigin(origin string) *webauthn.WebAuthn {
	if rpID, ok := s.originToRPID[origin]; ok {
		if webAuthn, ok := s.rpConfigs[rpID]; ok {
			logger.WithFields(map[string]interface{}{
				"origin": origin,
				"rp_id":  rpID,
			}).Info("Found WebAuthn instance for origin")
			return webAuthn
		}
	}
	// Fallback to primary
	logger.WithFields(map[string]interface{}{
		"origin":           origin,
		"available_origins": fmt.Sprintf("%v", s.getAvailableOrigins()),
	}).Warn("Origin not found in mapping, using primary WebAuthn")
	return s.webAuthn
}

// getAvailableOrigins returns all configured origins for debugging
func (s *WebAuthnService) getAvailableOrigins() []string {
	origins := make([]string, 0, len(s.originToRPID))
	for origin := range s.originToRPID {
		origins = append(origins, origin)
	}
	return origins
}

// GetWebAuthnForRPID returns the WebAuthn instance for a given RP ID
func (s *WebAuthnService) GetWebAuthnForRPID(rpID string) *webauthn.WebAuthn {
	if webAuthn, ok := s.rpConfigs[rpID]; ok {
		return webAuthn
	}
	// Fallback to primary
	return s.webAuthn
}

// GetDefault returns the default/primary WebAuthn instance
func (s *WebAuthnService) GetDefault() *webauthn.WebAuthn {
	return s.webAuthn
}

// parseCommaSeparated splits a comma-separated string and trims whitespace
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// generateAndroidOrigin creates an Android origin from a SHA256 fingerprint
// Input format: "4D:73:1B:48:..." or "4d731b48..." (hex with or without colons)
// Output format: "android:apk-key-hash:TXMbSAx3..."  (base64url encoded)
func generateAndroidOrigin(sha256Fingerprint string) string {
	// Remove colons and convert to lowercase
	hexStr := strings.ReplaceAll(sha256Fingerprint, ":", "")
	hexStr = strings.ToLower(hexStr)

	// Decode hex to bytes
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		logger.WithError(err).Error("Failed to decode SHA256 fingerprint")
		return ""
	}

	// Encode to base64url (no padding)
	b64 := base64.RawURLEncoding.EncodeToString(bytes)

	return "android:apk-key-hash:" + b64
}

// extractHostname extracts hostname from an origin URL
func extractHostname(origin string) string {
	// Remove protocol
	hostname := origin
	if idx := strings.Index(hostname, "://"); idx != -1 {
		hostname = hostname[idx+3:]
	}
	// Remove port if present
	if idx := strings.Index(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}
	// Remove path if present
	if idx := strings.Index(hostname, "/"); idx != -1 {
		hostname = hostname[:idx]
	}
	return hostname
}

// findMatchingRPID finds the best matching RP ID for a hostname
// It checks if hostname matches or is a subdomain of any RP ID
func findMatchingRPID(hostname string, rpIDs []string) string {
	// First, check for exact match
	for _, rpID := range rpIDs {
		if hostname == rpID {
			return rpID
		}
	}

	// Then, check if hostname is a subdomain of any RP ID
	for _, rpID := range rpIDs {
		if strings.HasSuffix(hostname, "."+rpID) {
			return rpID
		}
	}

	// Finally, check if hostname ends with RP ID (subdomain match)
	for _, rpID := range rpIDs {
		if hostname == rpID || strings.HasSuffix(hostname, "."+rpID) {
			return rpID
		}
	}

	// Default to hostname itself as RP ID
	return hostname
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
