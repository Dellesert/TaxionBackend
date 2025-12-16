package errors

import (
	"encoding/json"
	"net/http"
)

// ErrorCode represents a unique error code for API errors
type ErrorCode string

// Authentication error codes
const (
	// General authentication errors
	AuthInvalidCredentials    ErrorCode = "AUTH_INVALID_CREDENTIALS"
	AuthAccountDeactivated    ErrorCode = "AUTH_ACCOUNT_DEACTIVATED"
	AuthPasswordExpired       ErrorCode = "AUTH_PASSWORD_EXPIRED"
	AuthSessionExpired        ErrorCode = "AUTH_SESSION_EXPIRED"
	AuthTokenExpired          ErrorCode = "AUTH_TOKEN_EXPIRED"
	AuthTokenInvalid          ErrorCode = "AUTH_TOKEN_INVALID"
	AuthUnauthorized          ErrorCode = "AUTH_UNAUTHORIZED"

	// 2FA related errors
	Auth2FARequired           ErrorCode = "AUTH_2FA_REQUIRED"
	Auth2FANotEnabled         ErrorCode = "AUTH_2FA_NOT_ENABLED"
	Auth2FAInvalidCode        ErrorCode = "AUTH_2FA_INVALID_CODE"
	Auth2FACodeExpired        ErrorCode = "AUTH_2FA_CODE_EXPIRED"
	Auth2FASendFailed         ErrorCode = "AUTH_2FA_SEND_FAILED"

	// Passkey related errors
	AuthPasskeyOnly           ErrorCode = "AUTH_PASSKEY_ONLY"
	AuthPasskeyInvalid        ErrorCode = "AUTH_PASSKEY_INVALID"
	AuthPasskeyNotFound       ErrorCode = "AUTH_PASSKEY_NOT_FOUND"
	AuthPasskeyRegistrationFailed ErrorCode = "AUTH_PASSKEY_REGISTRATION_FAILED"

	// Access restrictions
	AuthSuperAdminWebOnly     ErrorCode = "AUTH_SUPER_ADMIN_WEB_ONLY"
	AuthInsufficientPermissions ErrorCode = "AUTH_INSUFFICIENT_PERMISSIONS"
	AuthPasswordLoginDisabled ErrorCode = "AUTH_PASSWORD_LOGIN_DISABLED"
)

// Validation error codes
const (
	ValidationFailed          ErrorCode = "VALIDATION_FAILED"
	ValidationRequiredField   ErrorCode = "VALIDATION_REQUIRED_FIELD"
	ValidationInvalidFormat   ErrorCode = "VALIDATION_INVALID_FORMAT"
	ValidationInvalidEmail    ErrorCode = "VALIDATION_INVALID_EMAIL"
	ValidationPasswordTooShort ErrorCode = "VALIDATION_PASSWORD_TOO_SHORT"
	ValidationPasswordTooWeak ErrorCode = "VALIDATION_PASSWORD_TOO_WEAK"
	ValidationInvalidRole     ErrorCode = "VALIDATION_INVALID_ROLE"
)

// User management error codes
const (
	UserNotFound              ErrorCode = "USER_NOT_FOUND"
	UserAlreadyExists         ErrorCode = "USER_ALREADY_EXISTS"
	UserCreationFailed        ErrorCode = "USER_CREATION_FAILED"
	UserUpdateFailed          ErrorCode = "USER_UPDATE_FAILED"
	UserDeletionFailed        ErrorCode = "USER_DELETION_FAILED"
)

// General error codes
const (
	InternalServerError       ErrorCode = "INTERNAL_SERVER_ERROR"
	BadRequest                ErrorCode = "BAD_REQUEST"
	NotFound                  ErrorCode = "NOT_FOUND"
	Forbidden                 ErrorCode = "FORBIDDEN"
	DatabaseError             ErrorCode = "DATABASE_ERROR"
)

// File management error codes
const (
	FileNotFound              ErrorCode = "FILE_NOT_FOUND"
	FileUploadFailed          ErrorCode = "FILE_UPLOAD_FAILED"
	FileDeleteFailed          ErrorCode = "FILE_DELETE_FAILED"
	FileAccessDenied          ErrorCode = "FILE_ACCESS_DENIED"
	FileInvalidType           ErrorCode = "FILE_INVALID_TYPE"
	FileTooLarge              ErrorCode = "FILE_TOO_LARGE"
	FileNoFileProvided        ErrorCode = "FILE_NO_FILE_PROVIDED"
	FileInvalidFormat         ErrorCode = "FILE_INVALID_FORMAT"
	FileThumbnailNotAvailable ErrorCode = "FILE_THUMBNAIL_NOT_AVAILABLE"
)

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string    `json:"field"`
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// APIError represents a structured API error response
type APIError struct {
	Error       string                 `json:"error"`
	ErrorCode   ErrorCode              `json:"error_code"`
	RequestID   string                 `json:"request_id,omitempty"`
	Details     interface{}            `json:"details,omitempty"`
	Fields      []FieldError           `json:"fields,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	StatusCode  int                    `json:"-"` // Not sent in JSON
}

// NewAPIError creates a new APIError
func NewAPIError(statusCode int, errorCode ErrorCode, message string) *APIError {
	return &APIError{
		Error:      message,
		ErrorCode:  errorCode,
		StatusCode: statusCode,
		Metadata:   make(map[string]interface{}),
	}
}

// WithRequestID adds request ID to the error
func (e *APIError) WithRequestID(requestID string) *APIError {
	e.RequestID = requestID
	return e
}

// WithDetails adds additional details to the error
func (e *APIError) WithDetails(details interface{}) *APIError {
	e.Details = details
	return e
}

// WithField adds a field validation error
func (e *APIError) WithField(field string, code ErrorCode, message string) *APIError {
	if e.Fields == nil {
		e.Fields = []FieldError{}
	}
	e.Fields = append(e.Fields, FieldError{
		Field:   field,
		Code:    code,
		Message: message,
	})
	return e
}

// WithMetadata adds metadata to the error
func (e *APIError) WithMetadata(key string, value interface{}) *APIError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// ToJSON converts the error to JSON
func (e *APIError) ToJSON() []byte {
	// Remove empty fields before marshaling
	response := map[string]interface{}{
		"error":      e.Error,
		"error_code": e.ErrorCode,
	}

	if e.RequestID != "" {
		response["request_id"] = e.RequestID
	}
	if e.Details != nil {
		response["details"] = e.Details
	}
	if len(e.Fields) > 0 {
		response["fields"] = e.Fields
	}
	if len(e.Metadata) > 0 {
		response["metadata"] = e.Metadata
	}

	jsonData, _ := json.Marshal(response)
	return jsonData
}

// Common error constructors

// BadRequestError creates a 400 Bad Request error
func BadRequestError(message string) *APIError {
	return NewAPIError(http.StatusBadRequest, BadRequest, message)
}

// UnauthorizedError creates a 401 Unauthorized error
func UnauthorizedError(message string) *APIError {
	return NewAPIError(http.StatusUnauthorized, AuthUnauthorized, message)
}

// ForbiddenError creates a 403 Forbidden error
func ForbiddenError(message string) *APIError {
	return NewAPIError(http.StatusForbidden, Forbidden, message)
}

// NotFoundError creates a 404 Not Found error
func NotFoundError(message string) *APIError {
	return NewAPIError(http.StatusNotFound, NotFound, message)
}

// InternalError creates a 500 Internal Server Error
func InternalError(message string) *APIError {
	return NewAPIError(http.StatusInternalServerError, InternalServerError, message)
}

// Authentication specific errors

// InvalidCredentialsError creates an invalid credentials error
func InvalidCredentialsError() *APIError {
	return NewAPIError(http.StatusUnauthorized, AuthInvalidCredentials, "Invalid email or password")
}

// AccountDeactivatedError creates an account deactivated error
func AccountDeactivatedError() *APIError {
	return NewAPIError(http.StatusForbidden, AuthAccountDeactivated, "Account is deactivated")
}

// TwoFactorRequiredError creates a 2FA required error
func TwoFactorRequiredError() *APIError {
	return NewAPIError(http.StatusForbidden, Auth2FARequired, "Two-factor authentication is required").
		WithMetadata("next_step", "send_2fa_code").
		WithMetadata("endpoint", "/api/v1/auth/2fa/send")
}

// PasskeyOnlyError creates a passkey-only mode error
func PasskeyOnlyError() *APIError {
	return NewAPIError(http.StatusForbidden, AuthPasskeyOnly, "Password login is disabled. Please use Passkey authentication").
		WithMetadata("available_methods", []string{"passkey"}).
		WithMetadata("endpoints", map[string]string{
			"passkey_login_begin":        "/api/v1/auth/passkey/login/begin",
			"passkey_login_discoverable": "/api/v1/auth/passkey/login/discoverable/begin",
		})
}

// SuperAdminWebOnlyError creates a super admin web-only error
func SuperAdminWebOnlyError() *APIError {
	return NewAPIError(http.StatusForbidden, AuthSuperAdminWebOnly, "Super admin access is restricted to web dashboard only")
}

// PasswordExpiredError creates a password expired error
func PasswordExpiredError() *APIError {
	return NewAPIError(http.StatusForbidden, AuthPasswordExpired, "Password has expired. Please change your password").
		WithMetadata("must_change_password", true)
}

// Validation specific errors

// RequiredFieldError creates a required field validation error
func RequiredFieldError(field string) *APIError {
	return NewAPIError(http.StatusBadRequest, ValidationRequiredField, "Validation failed").
		WithField(field, ValidationRequiredField, field+" is required")
}

// InvalidEmailError creates an invalid email validation error
func InvalidEmailError() *APIError {
	return NewAPIError(http.StatusBadRequest, ValidationInvalidEmail, "Invalid email format").
		WithField("email", ValidationInvalidEmail, "Invalid email format")
}

// PasswordTooShortError creates a password too short validation error
func PasswordTooShortError(minLength int) *APIError {
	return NewAPIError(http.StatusBadRequest, ValidationPasswordTooShort, "Password is too short").
		WithField("password", ValidationPasswordTooShort, "Password must be at least "+string(rune(minLength))+" characters long").
		WithMetadata("min_length", minLength)
}

// PasswordTooWeakError creates a password too weak validation error
func PasswordTooWeakError() *APIError {
	return NewAPIError(http.StatusBadRequest, ValidationPasswordTooWeak, "Password is too weak").
		WithField("password", ValidationPasswordTooWeak, "Password must contain at least one letter and one number or symbol")
}

// File specific errors

// FileNotFoundError creates a file not found error
func FileNotFoundError() *APIError {
	return NewAPIError(http.StatusNotFound, FileNotFound, "Файл не найден")
}

// FileUploadFailedError creates a file upload failed error
func FileUploadFailedError(details string) *APIError {
	return NewAPIError(http.StatusInternalServerError, FileUploadFailed, "Не удалось загрузить файл").
		WithDetails(details)
}

// FileDeleteFailedError creates a file deletion failed error
func FileDeleteFailedError(details string) *APIError {
	return NewAPIError(http.StatusInternalServerError, FileDeleteFailed, "Не удалось удалить файл").
		WithDetails(details)
}

// FileAccessDeniedError creates a file access denied error
func FileAccessDeniedError() *APIError {
	return NewAPIError(http.StatusForbidden, FileAccessDenied, "Доступ к файлу запрещен")
}

// FileInvalidTypeError creates an invalid file type error
func FileInvalidTypeError(allowedTypes []string) *APIError {
	err := NewAPIError(http.StatusBadRequest, FileInvalidType, "Недопустимый тип файла")
	if len(allowedTypes) > 0 {
		err.WithMetadata("allowed_types", allowedTypes)
	}
	return err
}

// FileTooLargeError creates a file too large error
func FileTooLargeError(maxSize int64) *APIError {
	return NewAPIError(http.StatusBadRequest, FileTooLarge, "Файл слишком большой").
		WithMetadata("max_size_mb", maxSize/(1024*1024))
}

// FileNoFileProvidedError creates a no file provided error
func FileNoFileProvidedError() *APIError {
	return NewAPIError(http.StatusBadRequest, FileNoFileProvided, "Файл не был предоставлен")
}

// FileInvalidFormatError creates an invalid file format error
func FileInvalidFormatError(details string) *APIError {
	return NewAPIError(http.StatusBadRequest, FileInvalidFormat, "Недопустимый формат файла").
		WithDetails(details)
}

// FileThumbnailNotAvailableError creates a thumbnail not available error
func FileThumbnailNotAvailableError() *APIError {
	return NewAPIError(http.StatusNotFound, FileThumbnailNotAvailable, "Миниатюра недоступна")
}
