package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/logger"
)

// AppVersionUsecase defines the interface for app version business logic
type AppVersionUsecase interface {
	CreateAppVersion(req *models.CreateAppVersionRequest, file *multipart.FileHeader, uploadedByID uint) (*models.AppVersionResponse, error)
	GetAppVersion(id uint) (*models.AppVersionResponse, error)
	GetByPlatformAndVersion(platform models.AppPlatform, version string) (*models.AppVersion, error)
	GetLatestVersions() (*models.LatestVersionsResponse, error)
	GetLatestByPlatform(platform models.AppPlatform) (*models.AppVersionResponse, error)
	UpdateAppVersion(id uint, req *models.UpdateAppVersionRequest) (*models.AppVersionResponse, error)
	DeleteAppVersion(id uint) error
	ListAppVersions(filters map[string]interface{}, page, pageSize int) (*models.AppVersionListResponse, error)
	GetStats() (*models.AppVersionStatsResponse, error)
	ActivateVersion(id uint) error
	GetDownloadPath(id uint) (string, error)
	IncrementDownloadCount(id uint) error
}

// appVersionUsecase implements AppVersionUsecase interface
type appVersionUsecase struct {
	appVersionRepo repository.AppVersionRepository
	userRepo       repository.UserRepository
	storageBasePath string // Base path for storing app files
}

// NewAppVersionUsecase creates a new app version usecase
func NewAppVersionUsecase(
	appVersionRepo repository.AppVersionRepository,
	userRepo repository.UserRepository,
) AppVersionUsecase {
	// Get storage path from environment or use default
	storagePath := os.Getenv("APP_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "./storage/app-versions"
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		logger.WithFields(map[string]interface{}{
			"path":  storagePath,
			"error": err.Error(),
		}).Error("Failed to create app storage directory")
	}

	return &appVersionUsecase{
		appVersionRepo:  appVersionRepo,
		userRepo:        userRepo,
		storageBasePath: storagePath,
	}
}

// CreateAppVersion creates a new app version
func (u *appVersionUsecase) CreateAppVersion(
	req *models.CreateAppVersionRequest,
	file *multipart.FileHeader,
	uploadedByID uint,
) (*models.AppVersionResponse, error) {
	// Validate platform
	platform := models.AppPlatform(req.Platform)
	if !isValidPlatform(platform) {
		return nil, fmt.Errorf("invalid platform: %s", req.Platform)
	}

	// Validate version format (e.g., 1.0.0)
	if !isValidVersionFormat(req.Version) {
		return nil, fmt.Errorf("invalid version format: %s (expected format: X.Y.Z)", req.Version)
	}

	// Check if version already exists
	existing, err := u.appVersionRepo.GetByPlatformAndVersion(platform, req.Version)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("version %s already exists for platform %s", req.Version, platform)
	}

	// Validate uploader exists
	if _, err := u.userRepo.GetByID(uploadedByID); err != nil {
		return nil, fmt.Errorf("invalid uploader: %w", err)
	}

	appVersion := &models.AppVersion{
		Platform:     platform,
		Version:      strings.TrimSpace(req.Version),
		BuildNumber:  req.BuildNumber,
		Changelog:    strings.TrimSpace(req.Changelog),
		IsCritical:   req.IsCritical,
		IsActive:     true, // New versions are active by default
		ReleaseDate:  time.Now(),
		UploadedByID: uploadedByID,
	}

	// Handle iOS (no file upload, just store URL)
	if platform == models.AppPlatformIOS {
		if req.StoreURL == "" {
			return nil, fmt.Errorf("store_url is required for iOS platform")
		}
		appVersion.StoreURL = req.StoreURL
	} else {
		// Handle Windows/Android (require file upload)
		if file == nil {
			return nil, fmt.Errorf("file is required for %s platform", platform)
		}

		// Validate file extension
		ext := filepath.Ext(file.Filename)
		if platform == models.AppPlatformWindows && ext != ".exe" && ext != ".msix" {
			return nil, fmt.Errorf("invalid file extension for Windows: %s (expected .exe or .msix)", ext)
		}
		if platform == models.AppPlatformAndroid && ext != ".apk" {
			return nil, fmt.Errorf("invalid file extension for Android: %s (expected .apk)", ext)
		}

		// Save file and calculate checksum
		filePath, fileSize, checksum, err := u.saveFile(file, platform, req.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to save file: %w", err)
		}

		appVersion.FilePath = filePath
		appVersion.FileSize = fileSize
		appVersion.Checksum = checksum
	}

	// Save to database
	if err := u.appVersionRepo.Create(appVersion); err != nil {
		// If database save fails, clean up the file
		if appVersion.FilePath != "" {
			os.Remove(appVersion.FilePath)
		}
		return nil, fmt.Errorf("failed to create app version: %w", err)
	}

	// Deactivate other versions for this platform (only one active version per platform)
	if err := u.appVersionRepo.DeactivateOtherVersions(platform, appVersion.ID); err != nil {
		logger.WithFields(map[string]interface{}{
			"platform": platform,
			"version_id": appVersion.ID,
			"error":   err.Error(),
		}).Warn("Failed to deactivate other versions")
	}

	// Get version with relations
	versionWithRelations, err := u.appVersionRepo.GetByIDWithRelations(appVersion.ID)
	if err != nil {
		versionWithRelations = appVersion
	}

	return versionWithRelations.ToResponse(), nil
}

// GetAppVersion retrieves an app version by ID
func (u *appVersionUsecase) GetAppVersion(id uint) (*models.AppVersionResponse, error) {
	version, err := u.appVersionRepo.GetByIDWithRelations(id)
	if err != nil {
		return nil, err
	}
	return version.ToResponse(), nil
}

// GetByPlatformAndVersion retrieves a specific version by platform and version string
func (u *appVersionUsecase) GetByPlatformAndVersion(platform models.AppPlatform, version string) (*models.AppVersion, error) {
	if !isValidPlatform(platform) {
		return nil, fmt.Errorf("invalid platform: %s", platform)
	}

	if version == "" {
		return nil, fmt.Errorf("version is required")
	}

	return u.appVersionRepo.GetByPlatformAndVersion(platform, version)
}

// GetLatestVersions retrieves the latest active version for all platforms
func (u *appVersionUsecase) GetLatestVersions() (*models.LatestVersionsResponse, error) {
	versions, err := u.appVersionRepo.GetLatestVersions()
	if err != nil {
		return nil, err
	}

	response := &models.LatestVersionsResponse{}
	if windows, ok := versions[models.AppPlatformWindows]; ok {
		response.Windows = windows.ToResponse()
	}
	if android, ok := versions[models.AppPlatformAndroid]; ok {
		response.Android = android.ToResponse()
	}
	if ios, ok := versions[models.AppPlatformIOS]; ok {
		response.IOS = ios.ToResponse()
	}

	return response, nil
}

// GetLatestByPlatform retrieves the latest version for a specific platform
func (u *appVersionUsecase) GetLatestByPlatform(platform models.AppPlatform) (*models.AppVersionResponse, error) {
	if !isValidPlatform(platform) {
		return nil, fmt.Errorf("invalid platform: %s", platform)
	}

	version, err := u.appVersionRepo.GetLatestByPlatform(platform)
	if err != nil {
		return nil, err
	}

	return version.ToResponse(), nil
}

// UpdateAppVersion updates app version metadata
func (u *appVersionUsecase) UpdateAppVersion(id uint, req *models.UpdateAppVersionRequest) (*models.AppVersionResponse, error) {
	version, err := u.appVersionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Changelog != nil {
		version.Changelog = *req.Changelog
	}
	if req.IsCritical != nil {
		version.IsCritical = *req.IsCritical
	}
	if req.IsActive != nil {
		wasActive := version.IsActive
		version.IsActive = *req.IsActive

		// If activating this version, deactivate others
		if !wasActive && *req.IsActive {
			if err := u.appVersionRepo.DeactivateOtherVersions(version.Platform, version.ID); err != nil {
				logger.WithFields(map[string]interface{}{
					"platform":   version.Platform,
					"version_id": version.ID,
					"error":      err.Error(),
				}).Warn("Failed to deactivate other versions")
			}
		}
	}
	if req.StoreURL != nil {
		version.StoreURL = *req.StoreURL
	}
	if req.BuildNumber != nil {
		version.BuildNumber = *req.BuildNumber
	}

	if err := u.appVersionRepo.Update(version); err != nil {
		return nil, fmt.Errorf("failed to update app version: %w", err)
	}

	// Get updated version with relations
	updatedVersion, err := u.appVersionRepo.GetByIDWithRelations(id)
	if err != nil {
		updatedVersion = version
	}

	return updatedVersion.ToResponse(), nil
}

// DeleteAppVersion deletes an app version and its file
func (u *appVersionUsecase) DeleteAppVersion(id uint) error {
	version, err := u.appVersionRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Delete file from disk if exists
	if version.FilePath != "" {
		if err := os.Remove(version.FilePath); err != nil {
			logger.WithFields(map[string]interface{}{
				"file_path": version.FilePath,
				"error":     err.Error(),
			}).Warn("Failed to delete file from disk")
		}
	}

	// Delete from database
	if err := u.appVersionRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete app version: %w", err)
	}

	return nil
}

// ListAppVersions retrieves a paginated list of app versions
func (u *appVersionUsecase) ListAppVersions(filters map[string]interface{}, page, pageSize int) (*models.AppVersionListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	versions, total, err := u.appVersionRepo.List(filters, page, pageSize)
	if err != nil {
		return nil, err
	}

	// Convert to response format
	versionResponses := make([]*models.AppVersionResponse, len(versions))
	for i, v := range versions {
		versionResponses[i] = v.ToResponse()
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	return &models.AppVersionListResponse{
		Versions:   versionResponses,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetStats retrieves statistics about app versions
func (u *appVersionUsecase) GetStats() (*models.AppVersionStatsResponse, error) {
	return u.appVersionRepo.GetStats()
}

// ActivateVersion activates a specific version and deactivates others for that platform
func (u *appVersionUsecase) ActivateVersion(id uint) error {
	version, err := u.appVersionRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Deactivate other versions
	if err := u.appVersionRepo.DeactivateOtherVersions(version.Platform, id); err != nil {
		return fmt.Errorf("failed to deactivate other versions: %w", err)
	}

	// Activate this version
	version.IsActive = true
	if err := u.appVersionRepo.Update(version); err != nil {
		return fmt.Errorf("failed to activate version: %w", err)
	}

	return nil
}

// GetDownloadPath returns the file path for downloading
func (u *appVersionUsecase) GetDownloadPath(id uint) (string, error) {
	version, err := u.appVersionRepo.GetByID(id)
	if err != nil {
		return "", err
	}

	if version.FilePath == "" {
		return "", fmt.Errorf("no file available for download")
	}

	// Check if file exists
	if _, err := os.Stat(version.FilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found on disk")
	}

	return version.FilePath, nil
}

// IncrementDownloadCount increments the download count for a version
func (u *appVersionUsecase) IncrementDownloadCount(id uint) error {
	return u.appVersionRepo.IncrementDownloadCount(id)
}

// Helper functions

// saveFile saves the uploaded file to disk and returns file path, size, and checksum
func (u *appVersionUsecase) saveFile(file *multipart.FileHeader, platform models.AppPlatform, version string) (string, int64, string, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create platform directory
	platformDir := filepath.Join(u.storageBasePath, string(platform))
	if err := os.MkdirAll(platformDir, 0755); err != nil {
		return "", 0, "", fmt.Errorf("failed to create platform directory: %w", err)
	}

	// Generate file name with version
	ext := filepath.Ext(file.Filename)
	fileName := fmt.Sprintf("%s-%s%s", string(platform), version, ext)
	filePath := filepath.Join(platformDir, fileName)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", 0, "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Calculate checksum while copying
	hash := sha256.New()
	multiWriter := io.MultiWriter(dst, hash)

	fileSize, err := io.Copy(multiWriter, src)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return "", 0, "", fmt.Errorf("failed to save file: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	return filePath, fileSize, checksum, nil
}

// isValidPlatform checks if the platform is valid
func isValidPlatform(platform models.AppPlatform) bool {
	switch platform {
	case models.AppPlatformWindows, models.AppPlatformAndroid, models.AppPlatformIOS:
		return true
	}
	return false
}

// isValidVersionFormat validates version string format (e.g., 1.0.0)
func isValidVersionFormat(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
