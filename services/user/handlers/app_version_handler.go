package handlers

import (
	"net/http"
	"os"
	"path/filepath"
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

// AppVersionHandler handles HTTP requests for app versions
type AppVersionHandler struct {
	appVersionUsecase usecase.AppVersionUsecase
}

// NewAppVersionHandler creates a new app version handler
func NewAppVersionHandler(appVersionUsecase usecase.AppVersionUsecase) *AppVersionHandler {
	return &AppVersionHandler{
		appVersionUsecase: appVersionUsecase,
	}
}

// CreateAppVersion handles app version creation (admin only)
// @Summary Create new app version
// @Tags App Versions
// @Accept multipart/form-data
// @Produce json
// @Param platform formData string true "Platform (windows, android, ios)"
// @Param version formData string true "Version (e.g., 1.0.0)"
// @Param changelog formData string false "Changelog"
// @Param is_critical formData boolean false "Is critical update"
// @Param store_url formData string false "App Store URL (for iOS only)"
// @Param file formData file false "Application file (required for Windows/Android)"
// @Success 201 {object} models.AppVersionResponse
// @Router /admin/app-versions [post]
func (h *AppVersionHandler) CreateAppVersion(c *gin.Context) {
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
	if err != nil || (userRole != sharedmodels.RoleSuperAdmin && userRole != sharedmodels.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только администраторы могут создавать версии приложения",
			"request_id": requestID,
		})
		return
	}

	// Parse form data
	var req models.CreateAppVersionRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверные данные запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Get file from request (may be nil for iOS)
	file, _ := c.FormFile("file")

	// Create app version
	version, err := h.appVersionUsecase.CreateAppVersion(&req, file, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"platform":   req.Platform,
			"version":    req.Version,
			"error":      err.Error(),
		}).Error("Failed to create app version")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось создать версию приложения"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "required") {
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
		"request_id": requestID,
		"user_id":    userID,
		"version_id": version.ID,
		"platform":   version.Platform,
		"version":    version.Version,
	}).Info("App version created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":     "App version created successfully",
		"app_version": version,
		"request_id":  requestID,
	})
}

// ListAppVersions handles listing app versions (admin only)
// @Summary List app versions
// @Tags App Versions
// @Produce json
// @Param platform query string false "Filter by platform"
// @Param is_active query boolean false "Filter by active status"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Success 200 {object} models.AppVersionListResponse
// @Router /admin/app-versions [get]
func (h *AppVersionHandler) ListAppVersions(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || (userRole != sharedmodels.RoleSuperAdmin && userRole != sharedmodels.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только администраторы могут просматривать версии приложения",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	filters := make(map[string]interface{})

	if platform := c.Query("platform"); platform != "" {
		filters["platform"] = platform
	}

	if isActive := c.Query("is_active"); isActive != "" {
		if isActive == "true" {
			filters["is_active"] = true
		} else if isActive == "false" {
			filters["is_active"] = false
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	// Get versions
	result, err := h.appVersionUsecase.ListAppVersions(filters, page, pageSize)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to list app versions")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить версии приложения",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetAppVersion handles getting a specific app version (admin only)
// @Summary Get app version by ID
// @Tags App Versions
// @Produce json
// @Param id path int true "Version ID"
// @Success 200 {object} models.AppVersionResponse
// @Router /admin/app-versions/{id} [get]
func (h *AppVersionHandler) GetAppVersion(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || (userRole != sharedmodels.RoleSuperAdmin && userRole != sharedmodels.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только администраторы могут просматривать версии приложения",
			"request_id": requestID,
		})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID версии",
			"request_id": requestID,
		})
		return
	}

	version, err := h.appVersionUsecase.GetAppVersion(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"version_id": id,
			"error":      err.Error(),
		}).Error("Failed to get app version")

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

	c.JSON(http.StatusOK, version)
}

// UpdateAppVersion handles updating app version metadata (admin only)
// @Summary Update app version
// @Tags App Versions
// @Accept json
// @Produce json
// @Param id path int true "Version ID"
// @Param request body models.UpdateAppVersionRequest true "Update request"
// @Success 200 {object} models.AppVersionResponse
// @Router /admin/app-versions/{id} [put]
func (h *AppVersionHandler) UpdateAppVersion(c *gin.Context) {
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
	if err != nil || (userRole != sharedmodels.RoleSuperAdmin && userRole != sharedmodels.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только администраторы могут обновлять версии приложения",
			"request_id": requestID,
		})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID версии",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateAppVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	version, err := h.appVersionUsecase.UpdateAppVersion(uint(id), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"version_id": id,
			"error":      err.Error(),
		}).Error("Failed to update app version")

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

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"version_id": id,
	}).Info("App version updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":     "App version updated successfully",
		"app_version": version,
		"request_id":  requestID,
	})
}

// DeleteAppVersion handles deleting an app version (admin only)
// @Summary Delete app version
// @Tags App Versions
// @Produce json
// @Param id path int true "Version ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/app-versions/{id} [delete]
func (h *AppVersionHandler) DeleteAppVersion(c *gin.Context) {
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
			"error":      "Только суперадмин может удалять версии приложения",
			"request_id": requestID,
		})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID версии",
			"request_id": requestID,
		})
		return
	}

	if err := h.appVersionUsecase.DeleteAppVersion(uint(id)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"version_id": id,
			"error":      err.Error(),
		}).Error("Failed to delete app version")

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

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"version_id": id,
	}).Info("App version deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "App version deleted successfully",
		"request_id": requestID,
	})
}

// ActivateVersion handles activating a specific version (admin only)
// @Summary Activate app version
// @Tags App Versions
// @Produce json
// @Param id path int true "Version ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/app-versions/{id}/activate [post]
func (h *AppVersionHandler) ActivateVersion(c *gin.Context) {
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
	if err != nil || (userRole != sharedmodels.RoleSuperAdmin && userRole != sharedmodels.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только администраторы могут активировать версии приложения",
			"request_id": requestID,
		})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID версии",
			"request_id": requestID,
		})
		return
	}

	if err := h.appVersionUsecase.ActivateVersion(uint(id)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"version_id": id,
			"error":      err.Error(),
		}).Error("Failed to activate app version")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"version_id": id,
	}).Info("App version activated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "App version activated successfully",
		"request_id": requestID,
	})
}

// GetStats handles getting app version statistics (admin only)
// @Summary Get app version statistics
// @Tags App Versions
// @Produce json
// @Success 200 {object} models.AppVersionStatsResponse
// @Router /admin/app-versions/stats [get]
func (h *AppVersionHandler) GetStats(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || (userRole != sharedmodels.RoleSuperAdmin && userRole != sharedmodels.RoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Только администраторы могут просматривать статистику",
			"request_id": requestID,
		})
		return
	}

	stats, err := h.appVersionUsecase.GetStats()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get app version stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить статистику",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Public endpoints (no authentication required)

// GetLatestVersions handles getting latest versions for all platforms (public)
// @Summary Get latest versions for all platforms
// @Tags App Versions
// @Produce json
// @Success 200 {object} models.LatestVersionsResponse
// @Router /api/v1/app-versions/latest [get]
func (h *AppVersionHandler) GetLatestVersions(c *gin.Context) {
	requestID := requestid.Get(c)

	versions, err := h.appVersionUsecase.GetLatestVersions()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get latest versions")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить последние версии",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, versions)
}

// GetLatestByPlatform handles getting latest version for a specific platform (public)
// @Summary Get latest version for platform
// @Tags App Versions
// @Produce json
// @Param platform path string true "Platform (windows, android, ios)"
// @Success 200 {object} models.AppVersionResponse
// @Router /api/v1/app-versions/latest/{platform} [get]
func (h *AppVersionHandler) GetLatestByPlatform(c *gin.Context) {
	requestID := requestid.Get(c)

	platform := models.AppPlatform(c.Param("platform"))

	version, err := h.appVersionUsecase.GetLatestByPlatform(platform)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"platform":   platform,
			"error":      err.Error(),
		}).Error("Failed to get latest version for platform")

		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "invalid") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, version)
}

// DownloadLatest handles downloading the latest version for a platform (public)
// @Summary Download latest version for platform
// @Tags App Versions
// @Produce application/octet-stream
// @Param platform path string true "Platform (windows, android)"
// @Success 200 {file} binary
// @Router /downloads/{platform}/latest [get]
func (h *AppVersionHandler) DownloadLatest(c *gin.Context) {
	requestID := requestid.Get(c)

	platform := models.AppPlatform(c.Param("platform"))

	// iOS doesn't support direct download
	if platform == models.AppPlatformIOS {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Приложения iOS необходимо загружать из App Store",
			"request_id": requestID,
		})
		return
	}

	version, err := h.appVersionUsecase.GetLatestByPlatform(platform)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	filePath, err := h.appVersionUsecase.GetDownloadPath(version.ID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"version_id": version.ID,
			"error":      err.Error(),
		}).Error("Failed to get download path")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Файл недоступен для скачивания",
			"request_id": requestID,
		})
		return
	}

	// Increment download count
	go h.appVersionUsecase.IncrementDownloadCount(version.ID)

	// Get filename from path
	filename := filepath.Base(filePath)

	// Set headers for download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")

	c.File(filePath)
}

// DownloadVersion handles downloading a specific version (public)
// @Summary Download specific version
// @Tags App Versions
// @Produce application/octet-stream
// @Param platform path string true "Platform (windows, android)"
// @Param version path string true "Version (e.g., 1.0.0)"
// @Success 200 {file} binary
// @Router /downloads/{platform}/{version} [get]
func (h *AppVersionHandler) DownloadVersion(c *gin.Context) {
	requestID := requestid.Get(c)

	platform := models.AppPlatform(c.Param("platform"))
	version := c.Param("version")

	// iOS doesn't support direct download
	if platform == models.AppPlatformIOS {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Приложения iOS необходимо загружать из App Store",
			"request_id": requestID,
		})
		return
	}

	// Validate platform
	if !isValidPlatform(platform) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверная платформа. Допустимые значения: 'windows' или 'android'",
			"request_id": requestID,
		})
		return
	}

	// Get specific version
	appVersion, err := h.appVersionUsecase.GetByPlatformAndVersion(platform, version)
	if err != nil {
		logger.Warnf("[%s] Version not found: platform=%s version=%s error=%v", requestID, platform, version, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Версия не найдена",
			"request_id": requestID,
		})
		return
	}

	// Check if file exists
	if appVersion.FilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Файл не найден",
			"request_id": requestID,
		})
		return
	}

	// Check if file exists on disk
	if _, err := os.Stat(appVersion.FilePath); os.IsNotExist(err) {
		logger.Errorf("[%s] File not found on disk: %s", requestID, appVersion.FilePath)
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Файл не найден на диске",
			"request_id": requestID,
		})
		return
	}

	// Increment download count
	go h.appVersionUsecase.IncrementDownloadCount(appVersion.ID)

	// Determine filename
	filename := filepath.Base(appVersion.FilePath)

	// Set headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")

	logger.Infof("[%s] Serving file: platform=%s version=%s file=%s", requestID, platform, version, filename)

	// Serve file
	c.File(appVersion.FilePath)
}

// Helper function to validate platform
func isValidPlatform(platform models.AppPlatform) bool {
	return platform == models.AppPlatformWindows ||
		platform == models.AppPlatformAndroid ||
		platform == models.AppPlatformIOS
}
