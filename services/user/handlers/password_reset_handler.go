package handlers

import (
	"net/http"
	"os"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PasswordResetHandler handles password reset related HTTP requests
type PasswordResetHandler struct {
	passwordResetUsecase usecase.PasswordResetUsecase
}

// NewPasswordResetHandler creates a new password reset handler
func NewPasswordResetHandler(passwordResetUsecase usecase.PasswordResetUsecase) *PasswordResetHandler {
	return &PasswordResetHandler{
		passwordResetUsecase: passwordResetUsecase,
	}
}

// InitiatePasswordReset handles admin request to initiate password reset (admin only)
func (h *PasswordResetHandler) InitiatePasswordReset(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get admin ID from context
	adminID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Check if user is admin or super_admin
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || (userRole != sharedmodels.RoleAdmin && userRole != sharedmodels.RoleSuperAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only admins can initiate password reset",
			"request_id": requestID,
		})
		return
	}

	var req models.InitiatePasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for initiate password reset")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Initiate password reset
	passwordReset, err := h.passwordResetUsecase.InitiatePasswordReset(req.UserID, &adminID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    req.UserID,
			"error":      err.Error(),
		}).Error("Failed to initiate password reset")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to initiate password reset",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":        requestID,
		"admin_id":          adminID,
		"user_id":           req.UserID,
		"password_reset_id": passwordReset.ID,
	}).Info("Password reset initiated")

	c.JSON(http.StatusOK, gin.H{
		"message":        "Password reset initiated successfully",
		"password_reset": passwordReset,
		"request_id":     requestID,
	})
}

// ValidateResetToken handles validation of password reset token (public endpoint)
func (h *PasswordResetHandler) ValidateResetToken(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Reset token is required",
			"request_id": requestID,
		})
		return
	}

	// Validate token
	passwordReset, err := h.passwordResetUsecase.ValidateResetToken(token)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid or expired reset token")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":        true,
		"reset_info":   passwordReset,
		"request_id":   requestID,
	})
}

// ResetPassword handles password reset using token (public endpoint)
func (h *PasswordResetHandler) ResetPassword(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Reset token is required",
			"request_id": requestID,
		})
		return
	}

	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for reset password")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Reset password
	if err := h.passwordResetUsecase.ResetPassword(token, &req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to reset password")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
	}).Info("Password reset successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Password reset successfully",
		"request_id": requestID,
	})
}

// RequestPasswordReset handles self-service password reset request (public endpoint)
func (h *PasswordResetHandler) RequestPasswordReset(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for password reset request")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"email":      req.Email,
	}).Info("Password reset request received")

	// Request password reset by email (always returns success to prevent email enumeration)
	_ = h.passwordResetUsecase.RequestPasswordResetByEmail(req.Email)

	// Always return the same generic success message
	c.JSON(http.StatusOK, gin.H{
		"message":    "If an account with that email exists, you will receive password reset instructions",
		"request_id": requestID,
	})
}

// PasswordResetRedirect handles the password reset redirect page (public endpoint)
// This page detects the platform and redirects to the mobile app or shows instructions
func (h *PasswordResetHandler) PasswordResetRedirect(c *gin.Context) {
	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(getErrorPageHTML("Неверная ссылка", "Ссылка для сброса пароля недействительна.")))
		return
	}

	// Validate token
	passwordReset, err := h.passwordResetUsecase.ValidateResetToken(token)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Warn("Invalid or expired reset token for redirect page")

		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(getErrorPageHTML("Ссылка недействительна", "Срок действия ссылки истёк или она уже была использована.")))
		return
	}

	logger.WithFields(map[string]interface{}{
		"email": passwordReset.Email,
	}).Info("Password reset redirect page accessed")

	// Render redirect page with platform detection
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(getPasswordResetRedirectHTML(token)))
}

// getPasswordResetRedirectHTML returns HTML for the password reset redirect page
func getPasswordResetRedirectHTML(token string) string {
	// Get app store URLs from environment
	appStoreURL := os.Getenv("APP_STORE_URL")
	if appStoreURL == "" {
		appStoreURL = "https://apps.apple.com/app/tachyon-messenger/idXXXXXXXXXX"
	}
	googlePlayURL := os.Getenv("GOOGLE_PLAY_URL")
	if googlePlayURL == "" {
		googlePlayURL = "https://play.google.com/store/apps/details?id=com.tachyon.messenger"
	}
	return `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Сброс пароля - Tachyon Messenger</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            max-width: 500px;
            width: 100%;
            padding: 40px;
            text-align: center;
        }
        .logo {
            font-size: 48px;
            margin-bottom: 20px;
        }
        h1 {
            color: #333;
            font-size: 28px;
            margin-bottom: 10px;
        }
        .subtitle {
            color: #666;
            font-size: 16px;
            margin-bottom: 30px;
        }
        .spinner {
            width: 50px;
            height: 50px;
            margin: 30px auto;
            border: 5px solid #f3f3f3;
            border-top: 5px solid #667eea;
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .status {
            color: #666;
            font-size: 14px;
            margin: 20px 0;
        }
        .instructions {
            background: #f8f9fa;
            border-radius: 10px;
            padding: 20px;
            margin-top: 20px;
            text-align: left;
            display: none;
        }
        .instructions h3 {
            color: #333;
            font-size: 18px;
            margin-bottom: 15px;
            text-align: center;
        }
        .instructions ol {
            padding-left: 20px;
            color: #555;
        }
        .instructions li {
            margin: 10px 0;
            line-height: 1.6;
        }
        .store-buttons {
            display: flex;
            flex-direction: column;
            gap: 10px;
            margin-top: 20px;
        }
        .store-button {
            display: inline-block;
            padding: 12px 20px;
            background: #000;
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-weight: 500;
            transition: background 0.3s;
        }
        .store-button:hover {
            background: #333;
        }
        .app-store { background: #000; }
        .play-store { background: #01875f; }
        .play-store:hover { background: #016647; }
        .token-box {
            background: white;
            border: 2px dashed #667eea;
            border-radius: 8px;
            padding: 15px;
            margin: 20px 0;
            word-break: break-all;
            font-family: monospace;
            font-size: 12px;
            color: #667eea;
            display: none;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">🔑</div>
        <h1>Сброс пароля</h1>
        <p class="subtitle">Перенаправление в приложение...</p>

        <div class="spinner"></div>
        <p class="status" id="status">Определяем вашу платформу...</p>

        <div class="instructions" id="instructions">
            <h3>📱 Откройте приложение</h3>
            <ol>
                <li>Если у вас установлено приложение Tachyon Messenger, оно откроется автоматически</li>
                <li>Если приложение не открылось, скопируйте код ниже</li>
                <li>Откройте приложение Tachyon Messenger вручную</li>
                <li>Перейдите в раздел "Сброс пароля"</li>
                <li>Вставьте скопированный код</li>
            </ol>

            <div class="token-box" id="tokenBox">` + token + `</div>

            <div class="store-buttons" id="storeButtons" style="display: none;">
                <a href="` + appStoreURL + `" class="store-button app-store">📱 Скачать из App Store</a>
                <a href="` + googlePlayURL + `" class="store-button play-store">📱 Скачать из Google Play</a>
            </div>
        </div>
    </div>

    <script>
        const token = '` + token + `';
        const statusEl = document.getElementById('status');
        const instructionsEl = document.getElementById('instructions');
        const tokenBoxEl = document.getElementById('tokenBox');
        const storeButtonsEl = document.getElementById('storeButtons');

        function detectPlatform() {
            const ua = navigator.userAgent || navigator.vendor || window.opera;

            if (/iPad|iPhone|iPod/.test(ua) && !window.MSStream) {
                return 'ios';
            }
            if (/android/i.test(ua)) {
                return 'android';
            }
            return 'web';
        }

        function tryOpenApp() {
            const platform = detectPlatform();

            // Try to open the app with Universal/App Link
            let appUrl = '';

            if (platform === 'ios') {
                // Universal Link for iOS
                appUrl = 'tachyon://reset-password/' + token;
                statusEl.textContent = 'Открываем приложение на iOS...';
            } else if (platform === 'android') {
                // App Link for Android
                appUrl = 'tachyon://reset-password/' + token;
                statusEl.textContent = 'Открываем приложение на Android...';
            } else {
                // Web - show instructions immediately
                showInstructions('Откройте эту страницу на мобильном устройстве', true);
                return;
            }

            // Try to open the app
            window.location.href = appUrl;

            // If app didn't open after 2 seconds, show instructions
            setTimeout(() => {
                showInstructions('Приложение не установлено?', false);
            }, 2000);
        }

        function showInstructions(message, showStoreButtons) {
            statusEl.textContent = message;
            instructionsEl.style.display = 'block';
            tokenBoxEl.style.display = 'block';

            if (showStoreButtons) {
                storeButtonsEl.style.display = 'flex';
            }

            document.querySelector('.spinner').style.display = 'none';
        }

        // Copy token to clipboard when clicked
        tokenBoxEl.addEventListener('click', () => {
            navigator.clipboard.writeText(token).then(() => {
                const originalText = tokenBoxEl.textContent;
                tokenBoxEl.textContent = '✅ Скопировано!';
                setTimeout(() => {
                    tokenBoxEl.textContent = originalText;
                }, 2000);
            });
        });

        // Start the process
        tryOpenApp();
    </script>
</body>
</html>`
}

// getErrorPageHTML returns HTML for error page
func getErrorPageHTML(title, message string) string {
	return `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + title + ` - Tachyon Messenger</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            max-width: 500px;
            width: 100%;
            padding: 40px;
            text-align: center;
        }
        .icon {
            font-size: 64px;
            margin-bottom: 20px;
        }
        h1 {
            color: #333;
            font-size: 28px;
            margin-bottom: 15px;
        }
        p {
            color: #666;
            font-size: 16px;
            line-height: 1.6;
        }
        .back-link {
            display: inline-block;
            margin-top: 30px;
            padding: 12px 30px;
            background: #667eea;
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-weight: 500;
            transition: background 0.3s;
        }
        .back-link:hover {
            background: #5568d3;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">⚠️</div>
        <h1>` + title + `</h1>
        <p>` + message + `</p>
        <a href="#" onclick="window.close(); return false;" class="back-link">Закрыть</a>
    </div>
</body>
</html>`
}
