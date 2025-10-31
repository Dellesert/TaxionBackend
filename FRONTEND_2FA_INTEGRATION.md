# Frontend 2FA Integration Guide

## Overview

TaxionDashboard has been updated to support Two-Factor Authentication (2FA) via email codes. The frontend now supports both JWT-based (legacy) and session-based (stateful) authentication.

## Changes Made

### 1. Type Definitions ([src/types/index.ts](D:\Documents\GitHub\TaxionDashboard\src\types\index.ts))

Added new field to User interface:
```typescript
export interface User {
  // ... existing fields
  two_factor_enabled?: boolean; // NEW: Indicates if 2FA is enabled for this user
  // ... rest of fields
}
```

Added new response interfaces:
```typescript
// Response when sending 2FA code
export interface Send2FACodeResponse {
  message: string;
  email: string;
  request_id: string;
}

// Response when verifying 2FA code
export interface Verify2FACodeResponse {
  message: string;
  user: User;
  tokens: TokenPair;
  must_change_password?: boolean;
}

// Response for session-based login
export interface SessionLoginResponse {
  message: string;
  user: User;
  must_change_password?: boolean;
}
```

### 2. API Client ([src/api/client.ts](D:\Documents\GitHub\TaxionDashboard\src\api\client.ts))

Updated axios client to support session-based authentication:
```typescript
const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true, // NEW: Enable cookie-based sessions for all requests
});
```

### 3. Authentication API ([src/api/auth.ts](D:\Documents\GitHub\TaxionDashboard\src\api\auth.ts))

Added new 2FA methods:

```typescript
// Step 1: Send 2FA code to user's email
send2FACode: async (email: string, password: string): Promise<Send2FACodeResponse>

// Step 2: Verify code and establish session
verify2FACode: async (email: string, code: string): Promise<Verify2FACodeResponse>

// Session-based login for users without 2FA
loginSession: async (email: string, password: string): Promise<SessionLoginResponse>

// Updated logout to call backend endpoint
logout: async (): Promise<void>
```

### 4. Auth Store ([src/store/authStore.ts](D:\Documents\GitHub\TaxionDashboard\src\store\authStore.ts))

Added session-based authentication support:
```typescript
interface AuthState {
  // ... existing fields
  isSessionBased: boolean; // NEW: Track if using session-based auth
  setSessionBased: (value: boolean) => void; // NEW: Toggle session mode
}
```

### 5. Users API ([src/api/users.ts](D:\Documents\GitHub\TaxionDashboard\src\api\users.ts))

Added 2FA management endpoint (super_admin only):
```typescript
// Update user's 2FA status
updateUser2FAStatus: async (userId: number, enabled: boolean): Promise<User>
```

## Backend Configuration

### CORS Settings

The backend is already configured to allow credentials. CORS origins in `.env`:
```env
CORS_ORIGINS=http://localhost:3000,http://localhost:8080,http://localhost:8093,http://localhost:5173,...
```

### Available Endpoints

#### 2FA Authentication Flow:
- `POST /api/v1/auth/2fa/send` - Send verification code
- `POST /api/v1/auth/2fa/verify` - Verify code and login
- `POST /api/v1/auth/logout` - Logout and invalidate session

#### 2FA Management (super_admin only):
- `PUT /api/v1/admin/users/:id/2fa` - Enable/disable 2FA for user

## Implementation Example

### 2FA Login Flow

```typescript
import { authAPI } from './api/auth';
import { useAuthStore } from './store/authStore';

async function handle2FALogin(email: string, password: string, code?: string) {
  const { setUser, setSessionBased } = useAuthStore.getState();

  try {
    if (!code) {
      // Step 1: Send code to email
      const response = await authAPI.send2FACode(email, password);
      console.log(response.message); // "Verification code sent to your email"
      return { needsCode: true };
    } else {
      // Step 2: Verify code
      const response = await authAPI.verify2FACode(email, code);

      // Set user and enable session-based mode
      setUser(response.user);
      setSessionBased(true);

      return { success: true, user: response.user };
    }
  } catch (error) {
    console.error('2FA login failed:', error);
    throw error;
  }
}
```

### Managing User 2FA Status

```typescript
import { usersAPI } from './api/users';

async function toggleUser2FA(userId: number, enabled: boolean) {
  try {
    const updatedUser = await usersAPI.updateUser2FAStatus(userId, enabled);
    console.log(`2FA ${enabled ? 'enabled' : 'disabled'} for user:`, updatedUser);
    return updatedUser;
  } catch (error) {
    if (error.response?.status === 403) {
      console.error('Only super_admin can manage 2FA settings');
    }
    throw error;
  }
}
```

### Logout

```typescript
import { authAPI } from './api/auth';
import { useAuthStore } from './store/authStore';

async function handleLogout() {
  const { logout: clearStore } = useAuthStore.getState();

  try {
    // Call backend to invalidate session
    await authAPI.logout();
  } catch (error) {
    console.error('Logout error:', error);
  } finally {
    // Always clear local state
    clearStore();
  }
}
```

## Testing

### Manual Testing Steps

1. **Start the development server:**
   ```bash
   cd D:\Documents\GitHub\TaxionDashboard
   npm run dev
   ```

2. **Create a test user with 2FA enabled:**
   ```sql
   -- Connect to PostgreSQL
   docker exec tachyon-postgres psql -U tachyon_user -d tachyon_messenger

   -- Enable 2FA for a user
   UPDATE users SET two_factor_enabled=true WHERE email='test@example.com';
   ```

3. **Test the 2FA flow:**
   - Enter email and password
   - Click "Send Code" button
   - Check email (mishajackson@inbox.ru for testing)
   - Enter the 6-digit code
   - Verify successful login

4. **Test 2FA management (as super_admin):**
   - Login as super_admin
   - Navigate to user management
   - Toggle 2FA for a user
   - Verify the status is updated

### Automated Testing

```typescript
// Example test using vitest/jest
describe('2FA Authentication', () => {
  it('should send verification code', async () => {
    const response = await authAPI.send2FACode('test@example.com', 'password123');
    expect(response.message).toBe('Verification code sent to your email');
    expect(response.email).toBe('test@example.com');
  });

  it('should verify code and login', async () => {
    const response = await authAPI.verify2FACode('test@example.com', '123456');
    expect(response.user).toBeDefined();
    expect(response.user.email).toBe('test@example.com');
  });
});
```

## Security Considerations

1. **HTTP-Only Cookies**: Session cookies are set as `httpOnly` by the backend to prevent XSS attacks.

2. **CORS**: Configured to only allow specific origins. Update `CORS_ORIGINS` in backend `.env` for production.

3. **Code Expiration**: 2FA codes expire after 5 minutes and are single-use only.

4. **Role-Based Access**: Only `super_admin` can manage 2FA settings for users.

5. **HTTPS**: In production, ensure `secure` flag is set for cookies and use HTTPS.

## Troubleshooting

### Issue: "CORS error" or credentials not sent

**Solution**: Ensure `withCredentials: true` is set in axios config and backend CORS includes your origin.

### Issue: "Only super administrators can manage 2FA settings"

**Solution**: This endpoint is restricted to `super_admin` role only. Regular `admin` cannot access it.

### Issue: Session not persisting

**Solution**: Check that cookies are being set. Use browser DevTools → Application → Cookies to verify `session_id` cookie exists.

### Issue: 2FA code not received

**Solution**:
1. Check SMTP configuration in backend `.env`
2. Verify email is configured in UniSender
3. Check spam folder
4. View backend logs: `docker logs tachyon-user-service`

## Next Steps

### UI Components to Build:

1. **2FA Login Form**
   - Email/password inputs
   - "Send Code" button
   - Code input field (6 digits)
   - "Verify" button

2. **2FA Management UI** (for super_admin)
   - Toggle switch in user table
   - Visual indicator (badge/icon) for users with 2FA enabled
   - Confirmation dialog before enabling/disabling

3. **User Settings** (for users to manage their own 2FA)
   - Enable/disable 2FA option
   - QR code for backup codes (future enhancement)

### Example Component Structure:

```
src/
├── components/
│   ├── auth/
│   │   ├── LoginForm.tsx
│   │   ├── TwoFactorForm.tsx
│   │   └── LogoutButton.tsx
│   └── admin/
│       ├── UserList.tsx
│       ├── User2FAToggle.tsx
│       └── User2FABadge.tsx
├── pages/
│   ├── LoginPage.tsx
│   └── AdminUsersPage.tsx
└── hooks/
    ├── useAuth.ts
    └── use2FA.ts
```

## Additional Resources

- [Backend 2FA Setup Guide](./2FA_SETUP.md)
- [API Documentation](./API_2FA_MANAGEMENT.md)
- [Quick Start Guide](./QUICK_START_2FA.md)
