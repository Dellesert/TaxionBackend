package models

import (
	"time"

	"tachyon-messenger/shared/models"
)

// PasskeyCredential represents a WebAuthn passkey credential
type PasskeyCredential struct {
	models.BaseModel
	UserID          uint       `gorm:"not null;index" json:"user_id"`
	User            *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CredentialID    []byte     `gorm:"not null;uniqueIndex;size:1024" json:"credential_id"`
	PublicKey       []byte     `gorm:"not null;size:2048" json:"public_key"`
	AttestationType string     `gorm:"not null;size:50" json:"attestation_type"`
	AAGUID          []byte     `gorm:"size:16" json:"aaguid,omitempty"`
	SignCount       uint32     `gorm:"not null;default:0" json:"sign_count"`
	Name            string     `gorm:"size:100" json:"name"` // User-friendly name (e.g., "iPhone 15", "MacBook Pro")
	Transports      string     `gorm:"size:255" json:"transports,omitempty"` // JSON array: ["usb","nfc","ble","internal"]
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	BackupEligible  bool       `gorm:"not null;default:false" json:"backup_eligible"`
	BackupState     bool       `gorm:"not null;default:false" json:"backup_state"`
}

// TableName returns the table name for PasskeyCredential model
func (PasskeyCredential) TableName() string {
	return "passkey_credentials"
}

// PasskeyCredentialResponse represents passkey credential in API responses
type PasskeyCredentialResponse struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Transports  string     `json:"transports,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ToResponse converts PasskeyCredential to PasskeyCredentialResponse
func (p *PasskeyCredential) ToResponse() *PasskeyCredentialResponse {
	return &PasskeyCredentialResponse{
		ID:         p.ID,
		Name:       p.Name,
		Transports: p.Transports,
		LastUsedAt: p.LastUsedAt,
		CreatedAt:  p.CreatedAt,
	}
}

// RegisterPasskeyRequest represents request to begin passkey registration
type RegisterPasskeyRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100" validate:"required,min=1,max=100"`
}

// VerifyPasskeyRegistrationRequest represents request to complete passkey registration
type VerifyPasskeyRegistrationRequest struct {
	Name                string `json:"name" binding:"required,min=1,max=100"`
	AttestationResponse string `json:"attestation_response" binding:"required"` // JSON from client
}

// PasskeyLoginRequest represents request to begin passkey login
type PasskeyLoginRequest struct {
	Email string `json:"email" binding:"required,email" validate:"required,email"`
}

// VerifyPasskeyLoginRequest represents request to complete passkey login
type VerifyPasskeyLoginRequest struct {
	Email             string `json:"email" binding:"required,email"`
	AssertionResponse string `json:"assertion_response" binding:"required"` // JSON from client
}
