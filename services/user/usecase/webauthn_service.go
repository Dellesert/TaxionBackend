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

// WebAuthnService wraps the WebAuthn library
type WebAuthnService struct {
	webAuthn *webauthn.WebAuthn
}

// NewWebAuthnService creates a new WebAuthn service
func NewWebAuthnService() (*WebAuthnService, error) {
	rpID := os.Getenv("WEBAUTHN_RP_ID")
	if rpID == "" {
		rpID = "localhost" // Default for development
	}

	rpDisplayName := os.Getenv("WEBAUTHN_RP_DISPLAY_NAME")
	if rpDisplayName == "" {
		rpDisplayName = "Tachyon Messenger"
	}

	rpOrigin := os.Getenv("WEBAUTHN_RP_ORIGIN")
	if rpOrigin == "" {
		rpOrigin = "http://localhost:8081" // Default for development
	}

	// Parse comma-separated origins
	rpOrigins := strings.Split(rpOrigin, ",")
	for i, origin := range rpOrigins {
		rpOrigins[i] = strings.TrimSpace(origin)
	}

	wconfig := &webauthn.Config{
		RPDisplayName: rpDisplayName,
		RPID:          rpID,
		RPOrigins:     rpOrigins,
		AttestationPreference: protocol.PreferNoAttestation,
		// Default settings for registration - use preferred for resident keys
		// This allows both resident and non-resident keys
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyNotRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementPreferred, // Preferred, not required
			UserVerification:   protocol.VerificationPreferred,           // Preferred, not required
		},
	}

	web, err := webauthn.New(wconfig)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to create WebAuthn instance")
		return nil, fmt.Errorf("failed to create WebAuthn instance: %w", err)
	}

	return &WebAuthnService{
		webAuthn: web,
	}, nil
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
