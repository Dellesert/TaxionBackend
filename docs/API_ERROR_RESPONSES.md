# API Error Responses Documentation

## Overview

All authentication endpoints now return structured error responses with error codes for better frontend error handling.

## Error Response Format

```json
{
  "error": "Human-readable error message",
  "error_code": "MACHINE_READABLE_ERROR_CODE",
  "request_id": "unique-request-id",
  "details": "Optional additional details",
  "fields": [
    {
      "field": "email",
      "code": "VALIDATION_REQUIRED_FIELD",
      "message": "Email is required"
    }
  ],
  "metadata": {
    "key": "value"
  }
}
```

## Error Codes

### Authentication Errors

#### General Authentication
- `AUTH_INVALID_CREDENTIALS` - Invalid email or password
- `AUTH_ACCOUNT_DEACTIVATED` - User account is deactivated
- `AUTH_PASSWORD_EXPIRED` - Password has expired
- `AUTH_SESSION_EXPIRED` - Session has expired
- `AUTH_TOKEN_EXPIRED` - JWT token has expired
- `AUTH_TOKEN_INVALID` - JWT token is invalid
- `AUTH_UNAUTHORIZED` - User is not authenticated

#### 2FA Related
- `AUTH_2FA_REQUIRED` - Two-factor authentication is required
  - Metadata: `next_step`, `endpoint`
- `AUTH_2FA_NOT_ENABLED` - 2FA is not enabled for this account
- `AUTH_2FA_INVALID_CODE` - Invalid verification code
- `AUTH_2FA_CODE_EXPIRED` - Verification code has expired
- `AUTH_2FA_SEND_FAILED` - Failed to send verification code

#### Passkey Related
- `AUTH_PASSKEY_ONLY` - Only passkey authentication is allowed
  - Metadata: `available_methods`, `endpoints`
- `AUTH_PASSKEY_INVALID` - Invalid passkey credential
- `AUTH_PASSKEY_NOT_FOUND` - Passkey not found
- `AUTH_PASSKEY_REGISTRATION_FAILED` - Failed to register passkey

#### Access Restrictions
- `AUTH_SUPER_ADMIN_WEB_ONLY` - Super admin can only login via web dashboard
- `AUTH_INSUFFICIENT_PERMISSIONS` - Insufficient permissions
- `AUTH_PASSWORD_LOGIN_DISABLED` - Password login is disabled

### Validation Errors

- `VALIDATION_FAILED` - General validation failure
- `VALIDATION_REQUIRED_FIELD` - Required field is missing
- `VALIDATION_INVALID_FORMAT` - Invalid format
- `VALIDATION_INVALID_EMAIL` - Invalid email format
- `VALIDATION_PASSWORD_TOO_SHORT` - Password is too short
- `VALIDATION_PASSWORD_TOO_WEAK` - Password is too weak
- `VALIDATION_INVALID_ROLE` - Invalid role

### User Management Errors

- `USER_NOT_FOUND` - User not found
- `USER_ALREADY_EXISTS` - User already exists
- `USER_CREATION_FAILED` - Failed to create user
- `USER_UPDATE_FAILED` - Failed to update user
- `USER_DELETION_FAILED` - Failed to delete user

### General Errors

- `INTERNAL_SERVER_ERROR` - Internal server error
- `BAD_REQUEST` - Bad request
- `NOT_FOUND` - Resource not found
- `FORBIDDEN` - Access forbidden
- `DATABASE_ERROR` - Database error

## Endpoint Examples

### POST /api/v1/auth/login

#### Success Response (200 OK)
```json
{
  "message": "Login successful",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe",
    "role": "employee"
  },
  "auth_mode": "jwt",
  "tokens": {
    "access_token": "eyJhbGc...",
    "refresh_token": "eyJhbGc..."
  },
  "request_id": "abc-123"
}
```

#### Error: Invalid Credentials (401 Unauthorized)
```json
{
  "error": "Invalid email or password",
  "error_code": "AUTH_INVALID_CREDENTIALS",
  "request_id": "abc-123"
}
```

#### Error: Account Deactivated (403 Forbidden)
```json
{
  "error": "Account is deactivated",
  "error_code": "AUTH_ACCOUNT_DEACTIVATED",
  "request_id": "abc-123"
}
```

#### Error: 2FA Required (403 Forbidden)
```json
{
  "error": "Two-factor authentication is required",
  "error_code": "AUTH_2FA_REQUIRED",
  "request_id": "abc-123",
  "metadata": {
    "next_step": "send_2fa_code",
    "endpoint": "/api/v1/auth/2fa/send"
  }
}
```

#### Error: Passkey Only Mode (403 Forbidden)
```json
{
  "error": "Password login is disabled. Please use Passkey authentication",
  "error_code": "AUTH_PASSKEY_ONLY",
  "request_id": "abc-123",
  "metadata": {
    "available_methods": ["passkey"],
    "endpoints": {
      "passkey_login_begin": "/api/v1/auth/passkey/login/begin",
      "passkey_login_discoverable": "/api/v1/auth/passkey/login/discoverable/begin"
    }
  }
}
```

#### Error: Super Admin Web Only (403 Forbidden)
```json
{
  "error": "Super admin access is restricted to web dashboard only",
  "error_code": "AUTH_SUPER_ADMIN_WEB_ONLY",
  "request_id": "abc-123"
}
```

#### Error: Required Field (400 Bad Request)
```json
{
  "error": "Validation failed",
  "error_code": "VALIDATION_REQUIRED_FIELD",
  "request_id": "abc-123",
  "fields": [
    {
      "field": "email",
      "code": "VALIDATION_REQUIRED_FIELD",
      "message": "email is required"
    }
  ]
}
```

### POST /api/v1/auth/2fa/send

#### Success Response (200 OK)
```json
{
  "message": "Verification code sent to your email",
  "request_id": "abc-123",
  "code_expires_in": 300,
  "can_resend_after": 60
}
```

#### Error: 2FA Not Enabled (400 Bad Request)
```json
{
  "error": "Two-factor authentication is not enabled for this account",
  "error_code": "AUTH_2FA_NOT_ENABLED",
  "request_id": "abc-123"
}
```

### POST /api/v1/auth/2fa/verify

#### Success Response (200 OK)
```json
{
  "message": "Login successful",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe"
  },
  "auth_mode": "jwt",
  "tokens": {
    "access_token": "eyJhbGc...",
    "refresh_token": "eyJhbGc..."
  },
  "request_id": "abc-123"
}
```

#### Error: Invalid or Expired Code (401 Unauthorized)
```json
{
  "error": "Verification code is invalid or expired",
  "error_code": "AUTH_2FA_CODE_EXPIRED",
  "request_id": "abc-123"
}
```

### POST /api/v1/auth/passkey/login/begin

#### Error: Invalid Credentials (401 Unauthorized)
```json
{
  "error": "Invalid email or password",
  "error_code": "AUTH_INVALID_CREDENTIALS",
  "request_id": "abc-123"
}
```

### POST /api/v1/auth/passkey/login/finish

#### Error: Invalid Passkey (401 Unauthorized)
```json
{
  "error": "Invalid passkey",
  "error_code": "AUTH_PASSKEY_INVALID",
  "request_id": "abc-123"
}
```

### POST /api/v1/auth/passkey/register/begin

#### Error: Unauthorized (401 Unauthorized)
```json
{
  "error": "Unauthorized",
  "error_code": "AUTH_UNAUTHORIZED",
  "request_id": "abc-123"
}
```

#### Error: Registration Failed (400 Bad Request)
```json
{
  "error": "Failed to begin passkey registration",
  "error_code": "AUTH_PASSKEY_REGISTRATION_FAILED",
  "request_id": "abc-123",
  "details": "detailed error message"
}
```

## Frontend Integration Guide

### Error Handling Pattern

```typescript
async function handleLogin(email: string, password: string) {
  try {
    const response = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    });

    const data = await response.json();

    if (!response.ok) {
      // Handle error based on error_code
      switch (data.error_code) {
        case 'AUTH_INVALID_CREDENTIALS':
          showError('Неверный email или пароль');
          break;

        case 'AUTH_ACCOUNT_DEACTIVATED':
          showError('Аккаунт деактивирован. Обратитесь к администратору.');
          break;

        case 'AUTH_2FA_REQUIRED':
          // Redirect to 2FA page
          const endpoint = data.metadata?.endpoint;
          redirectTo2FA(email, endpoint);
          break;

        case 'AUTH_PASSKEY_ONLY':
          // Show passkey login options
          const endpoints = data.metadata?.endpoints;
          showPasskeyLogin(endpoints);
          break;

        case 'AUTH_SUPER_ADMIN_WEB_ONLY':
          showError('Используйте веб-панель для входа');
          break;

        case 'VALIDATION_REQUIRED_FIELD':
          // Handle field validation errors
          data.fields?.forEach(field => {
            showFieldError(field.field, field.message);
          });
          break;

        default:
          showError(data.error || 'Ошибка входа');
      }
      return;
    }

    // Success - handle login
    if (data.must_change_password) {
      redirectToChangePassword(data.tokens || data.session);
    } else {
      saveAuthData(data);
      redirectToDashboard();
    }

  } catch (error) {
    showError('Ошибка соединения с сервером');
  }
}
```

### 2FA Flow Example

```typescript
// Step 1: Send 2FA code
async function send2FACode(email: string, password: string) {
  const response = await fetch('/api/v1/auth/2fa/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  });

  const data = await response.json();

  if (response.ok) {
    // Show countdown timer
    const expiresIn = data.code_expires_in; // 300 seconds
    const canResendAfter = data.can_resend_after; // 60 seconds

    startCountdown(expiresIn);
    enableResendAfter(canResendAfter);

    showCodeInput();
  } else {
    handleError(data.error_code, data.error);
  }
}

// Step 2: Verify 2FA code
async function verify2FACode(email: string, code: string) {
  const response = await fetch('/api/v1/auth/2fa/verify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, code })
  });

  const data = await response.json();

  if (response.ok) {
    // Login successful
    saveAuthData(data);
    redirectToDashboard();
  } else {
    if (data.error_code === 'AUTH_2FA_CODE_EXPIRED') {
      showError('Код истёк. Запросите новый код.');
      showResendButton();
    } else {
      showError(data.error);
    }
  }
}
```

## Migration Notes

### For Existing Frontends

1. **Error Code Checking**: Update error handling to check `error_code` field instead of parsing `error` message text
2. **Metadata Usage**: Use `metadata` field for additional context (endpoints, available methods, etc.)
3. **Field Validation**: Handle `fields` array for detailed validation errors
4. **2FA Timing**: Use `code_expires_in` and `can_resend_after` for better UX
5. **Request Tracking**: Use `request_id` for debugging and support

### Backward Compatibility

All error responses include the `error` field with human-readable messages, so existing frontends will continue to work. However, it's recommended to migrate to using `error_code` for more reliable error handling.
