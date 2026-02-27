package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// InvitationHandler handles HTTP requests for invitations
type InvitationHandler struct {
	invitationUsecase usecase.InvitationUsecase
}

// NewInvitationHandler creates a new invitation handler
func NewInvitationHandler(invitationUsecase usecase.InvitationUsecase) *InvitationHandler {
	return &InvitationHandler{
		invitationUsecase: invitationUsecase,
	}
}

// CreateInvitation handles invitation creation (super_admin only)
func (h *InvitationHandler) CreateInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to get user ID for invitation creation")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Пользователь не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"user_role":  userRole,
		}).Warn("Unauthorized invitation creation attempt")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может создавать приглашения",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for invitation creation")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Create invitation
	invitation, err := h.invitationUsecase.CreateInvitation(&req, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Error("Failed to create invitation")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось создать приглашение"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "pending invitation") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"invitation_id": invitation.ID,
		"email":         invitation.Email,
	}).Info("Invitation created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Invitation created successfully",
		"invitation": invitation,
		"request_id": requestID,
	})
}

// ListInvitations handles listing invitations (super_admin only)
func (h *InvitationHandler) ListInvitations(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может просматривать приглашения",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	filters := make(map[string]interface{})

	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}

	if email := c.Query("email"); email != "" {
		filters["email"] = email
	}

	if role := c.Query("role"); role != "" {
		filters["role"] = role
	}

	if departmentIDStr := c.Query("department_id"); departmentIDStr != "" {
		if departmentID, err := strconv.ParseUint(departmentIDStr, 10, 32); err == nil {
			filters["department_id"] = uint(departmentID)
		}
	}

	if isValidStr := c.Query("is_valid"); isValidStr == "true" {
		filters["is_valid"] = true
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// List invitations
	response, err := h.invitationUsecase.ListInvitations(filters, page, pageSize)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to list invitations")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить список приглашений",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       response,
		"request_id": requestID,
	})
}

// GetInvitation handles getting a single invitation (super_admin only)
func (h *InvitationHandler) GetInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может просматривать приглашения",
			"request_id": requestID,
		})
		return
	}

	// Get invitation ID from URL
	invitationID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID приглашения",
			"request_id": requestID,
		})
		return
	}

	// Get invitation
	invitation, err := h.invitationUsecase.GetInvitation(uint(invitationID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"invitation_id": invitationID,
			"error":         err.Error(),
		}).Error("Failed to get invitation")

		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invitation": invitation,
		"request_id": requestID,
	})
}

// ResendInvitation handles resending an invitation (super_admin only)
func (h *InvitationHandler) ResendInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Пользователь не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может повторно отправлять приглашения",
			"request_id": requestID,
		})
		return
	}

	// Get invitation ID from URL
	invitationID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID приглашения",
			"request_id": requestID,
		})
		return
	}

	// Resend invitation
	invitation, err := h.invitationUsecase.ResendInvitation(uint(invitationID), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"user_id":       userID,
			"invitation_id": invitationID,
			"error":         err.Error(),
		}).Error("Failed to resend invitation")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось повторно отправить приглашение"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "can only resend") ||
			strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"invitation_id": invitationID,
	}).Info("Invitation resent successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Invitation resent successfully",
		"invitation": invitation,
		"request_id": requestID,
	})
}

// CancelInvitation handles canceling an invitation (super_admin only)
func (h *InvitationHandler) CancelInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Пользователь не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может отменять приглашения",
			"request_id": requestID,
		})
		return
	}

	// Get invitation ID from URL
	invitationID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID приглашения",
			"request_id": requestID,
		})
		return
	}

	// Cancel invitation
	err = h.invitationUsecase.CancelInvitation(uint(invitationID), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"user_id":       userID,
			"invitation_id": invitationID,
			"error":         err.Error(),
		}).Error("Failed to cancel invitation")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось отменить приглашение"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "can only cancel") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"invitation_id": invitationID,
	}).Info("Invitation cancelled successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Invitation cancelled successfully",
		"request_id": requestID,
	})
}

// GetStats handles getting invitation statistics (super_admin only)
func (h *InvitationHandler) GetStats(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может просматривать статистику приглашений",
			"request_id": requestID,
		})
		return
	}

	// Get stats
	stats, err := h.invitationUsecase.GetStats()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get invitation statistics")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить статистику",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"request_id": requestID,
	})
}

// ValidateInvitation handles invitation validation (public endpoint)
func (h *InvitationHandler) ValidateInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Токен обязателен",
			"request_id": requestID,
		})
		return
	}

	// Validate invitation
	invitation, err := h.invitationUsecase.ValidateInvitation(token)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid invitation validation attempt")

		statusCode := http.StatusBadRequest
		if strings.Contains(err.Error(), "expired") {
			statusCode = http.StatusGone
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":      true,
		"invitation": invitation,
		"request_id": requestID,
	})
}

// BulkSendInvitations handles bulk sending invitations to selected users (super_admin only)
func (h *InvitationHandler) BulkSendInvitations(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Пользователь не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только суперадмин может отправлять массовые приглашения",
			"request_id": requestID,
		})
		return
	}

	var req models.BulkSendInvitationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for bulk send invitations")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Validate user IDs list
	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Требуется хотя бы один ID пользователя",
			"request_id": requestID,
		})
		return
	}

	// Send invitations
	response, err := h.invitationUsecase.BulkSendInvitations(req.UserIDs, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"user_count": len(req.UserIDs),
			"error":      err.Error(),
		}).Error("Failed to send bulk invitations")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось отправить массовые приглашения",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"total_users":   response.TotalUsers,
		"success_count": response.SuccessCount,
		"error_count":   response.ErrorCount,
	}).Info("Bulk invitations sent")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Bulk invitations processed",
		"data":       response,
		"request_id": requestID,
	})
}

// InvitationRedirect handles the invitation redirect page (public endpoint)
// This page detects the platform and redirects to the mobile app or shows instructions
func (h *InvitationHandler) InvitationRedirect(c *gin.Context) {
	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(getInviteErrorPageHTML("Неверная ссылка", "Ссылка приглашения недействительна.")))
		return
	}

	// Validate invitation
	invitation, err := h.invitationUsecase.ValidateInvitation(token)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Warn("Invalid or expired invitation token for redirect page")

		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(getInviteErrorPageHTML("Приглашение недействительно", "Срок действия приглашения истёк или оно уже было использовано.")))
		return
	}

	logger.WithFields(map[string]interface{}{
		"email": invitation.Email,
	}).Info("Invitation redirect page accessed")

	// Render redirect page with platform detection
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(getInvitationRedirectHTML(token, invitation.Name)))
}

// getInvitationRedirectHTML returns HTML for the invitation redirect page
func getInvitationRedirectHTML(token, userName string) string {
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
    <title>Приглашение в Tachyon Messenger</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #E94444 0%, #c72c2c 100%);
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
        .user-name {
            color: #E94444;
            font-weight: bold;
        }
        .spinner {
            width: 50px;
            height: 50px;
            margin: 30px auto;
            border: 5px solid #f3f3f3;
            border-top: 5px solid #E94444;
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
            border: 2px dashed #E94444;
            border-radius: 8px;
            padding: 15px;
            margin: 20px 0;
            word-break: break-all;
            font-family: monospace;
            font-size: 12px;
            color: #E94444;
            display: none;
            cursor: pointer;
        }
        .token-box:hover {
            background: #fff5f5;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">📱</div>
        <h1>Добро пожаловать!</h1>
        <p class="subtitle">Приглашение в <strong>Tachyon Messenger</strong>` +
		(func() string {
			if userName != "" {
				return `<br><span class="user-name">` + userName + `</span>`
			}
			return ""
		})() + `</p>

        <div class="spinner"></div>
        <p class="status" id="status">Определяем вашу платформу...</p>

        <div class="instructions" id="instructions">
            <h3>📱 Откройте приложение</h3>
            <ol>
                <li>Если у вас установлено приложение Tachyon Messenger, оно откроется автоматически</li>
                <li>Если приложение не открылось, скопируйте код ниже</li>
                <li>Откройте приложение Tachyon Messenger вручную</li>
                <li>Нажмите "Есть приглашение?" или "У меня есть код"</li>
                <li>Вставьте скопированный код приглашения</li>
            </ol>

            <div class="token-box" id="tokenBox" title="Нажмите, чтобы скопировать">` + token + `</div>

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
                appUrl = 'tachyon://invite/' + token;
                statusEl.textContent = 'Открываем приложение на iOS...';
            } else if (platform === 'android') {
                // App Link for Android
                appUrl = 'tachyon://invite/' + token;
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

// getInviteErrorPageHTML returns HTML for invitation error page
func getInviteErrorPageHTML(title, message string) string {
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
            background: linear-gradient(135deg, #E94444 0%, #c72c2c 100%);
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
            background: #E94444;
            color: white;
            text-decoration: none;
            border-radius: 8px;
            font-weight: 500;
            transition: background 0.3s;
        }
        .back-link:hover {
            background: #c72c2c;
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

// AcceptInvitation handles invitation acceptance (public endpoint)
func (h *InvitationHandler) AcceptInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Токен обязателен",
			"request_id": requestID,
		})
		return
	}

	var req models.AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for invitation acceptance")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Extract client info for session tracking
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	// Accept invitation
	loginResponse, err := h.invitationUsecase.AcceptInvitation(token, &req, ipAddress, userAgent)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"token":      token,
			"error":      err.Error(),
		}).Error("Failed to accept invitation")

		statusCode := http.StatusBadRequest
		errorMessage := err.Error()

		if strings.Contains(err.Error(), "expired") {
			statusCode = http.StatusGone
		} else if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "no longer valid") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
		} else {
			statusCode = http.StatusInternalServerError
			errorMessage = "Не удалось принять приглашение"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	// Set session cookie if in session mode
	if loginResponse.Session != nil {
		c.SetCookie(
			"session_id",
			loginResponse.Session.SessionID,
			int(loginResponse.Session.ExpiresAt),
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			true,  // httpOnly
		)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    loginResponse.User.ID,
		"email":      loginResponse.User.Email,
	}).Info("Invitation accepted successfully, user created and logged in")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Account activated successfully",
		"user":       loginResponse.User,
		"auth_mode":  loginResponse.AuthMode,
		"session":    loginResponse.Session,
		"request_id": requestID,
	})
}
