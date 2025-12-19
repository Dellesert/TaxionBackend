package repository

import (
	"fmt"

	"tachyon-messenger/services/user/models"

	"gorm.io/gorm"
)

// AppVersionRepository defines the interface for app version data operations
type AppVersionRepository interface {
	Create(version *models.AppVersion) error
	GetByID(id uint) (*models.AppVersion, error)
	GetByIDWithRelations(id uint) (*models.AppVersion, error)
	GetByPlatformAndVersion(platform models.AppPlatform, version string) (*models.AppVersion, error)
	GetLatestByPlatform(platform models.AppPlatform) (*models.AppVersion, error)
	GetLatestVersions() (map[models.AppPlatform]*models.AppVersion, error)
	Update(version *models.AppVersion) error
	Delete(id uint) error
	List(filters map[string]interface{}, page, pageSize int) ([]*models.AppVersion, int64, error)
	GetStats() (*models.AppVersionStatsResponse, error)
	DeactivateOtherVersions(platform models.AppPlatform, exceptID uint) error
	IncrementDownloadCount(id uint) error
	GetAllByPlatform(platform models.AppPlatform) ([]*models.AppVersion, error)
}

// appVersionRepository implements AppVersionRepository interface
type appVersionRepository struct {
	db *gorm.DB
}

// NewAppVersionRepository creates a new app version repository
func NewAppVersionRepository(db *gorm.DB) AppVersionRepository {
	return &appVersionRepository{
		db: db,
	}
}

// Create creates a new app version
func (r *appVersionRepository) Create(version *models.AppVersion) error {
	return r.db.Create(version).Error
}

// GetByID retrieves an app version by ID
func (r *appVersionRepository) GetByID(id uint) (*models.AppVersion, error) {
	var version models.AppVersion
	err := r.db.Where("id = ?", id).First(&version).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("app version not found")
		}
		return nil, err
	}
	return &version, nil
}

// GetByIDWithRelations retrieves an app version with all relations (uploader)
func (r *appVersionRepository) GetByIDWithRelations(id uint) (*models.AppVersion, error) {
	var version models.AppVersion
	err := r.db.
		Preload("UploadedBy").
		Where("id = ?", id).
		First(&version).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("app version not found")
		}
		return nil, err
	}
	return &version, nil
}

// GetByPlatformAndVersion retrieves an app version by platform and version string
func (r *appVersionRepository) GetByPlatformAndVersion(platform models.AppPlatform, version string) (*models.AppVersion, error) {
	var appVersion models.AppVersion
	err := r.db.Where("platform = ? AND version = ?", platform, version).First(&appVersion).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("app version not found")
		}
		return nil, err
	}
	return &appVersion, nil
}

// GetLatestByPlatform retrieves the latest active version for a platform
func (r *appVersionRepository) GetLatestByPlatform(platform models.AppPlatform) (*models.AppVersion, error) {
	var version models.AppVersion
	err := r.db.
		Where("platform = ? AND is_active = ?", platform, true).
		Order("release_date DESC").
		First(&version).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no active version found for platform %s", platform)
		}
		return nil, err
	}
	return &version, nil
}

// GetLatestVersions retrieves the latest active version for all platforms
func (r *appVersionRepository) GetLatestVersions() (map[models.AppPlatform]*models.AppVersion, error) {
	result := make(map[models.AppPlatform]*models.AppVersion)

	platforms := []models.AppPlatform{
		models.AppPlatformWindows,
		models.AppPlatformAndroid,
		models.AppPlatformIOS,
	}

	for _, platform := range platforms {
		version, err := r.GetLatestByPlatform(platform)
		if err == nil {
			result[platform] = version
		}
		// If no version found for platform, just skip it (don't return error)
	}

	return result, nil
}

// Update updates an app version
func (r *appVersionRepository) Update(version *models.AppVersion) error {
	return r.db.Save(version).Error
}

// Delete deletes an app version
func (r *appVersionRepository) Delete(id uint) error {
	return r.db.Delete(&models.AppVersion{}, id).Error
}

// List retrieves a paginated list of app versions with optional filters
func (r *appVersionRepository) List(filters map[string]interface{}, page, pageSize int) ([]*models.AppVersion, int64, error) {
	var versions []*models.AppVersion
	var total int64

	query := r.db.Model(&models.AppVersion{}).Preload("UploadedBy")

	// Apply filters
	if platform, ok := filters["platform"]; ok {
		query = query.Where("platform = ?", platform)
	}
	if isActive, ok := filters["is_active"]; ok {
		query = query.Where("is_active = ?", isActive)
	}
	if isCritical, ok := filters["is_critical"]; ok {
		query = query.Where("is_critical = ?", isCritical)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * pageSize
	if err := query.
		Order("release_date DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&versions).Error; err != nil {
		return nil, 0, err
	}

	return versions, total, nil
}

// GetStats retrieves statistics about app versions
func (r *appVersionRepository) GetStats() (*models.AppVersionStatsResponse, error) {
	stats := &models.AppVersionStatsResponse{}

	// Total versions
	r.db.Model(&models.AppVersion{}).Count(&stats.TotalVersions)

	// Platform-specific counts
	r.db.Model(&models.AppVersion{}).Where("platform = ?", models.AppPlatformWindows).Count(&stats.WindowsVersions)
	r.db.Model(&models.AppVersion{}).Where("platform = ?", models.AppPlatformAndroid).Count(&stats.AndroidVersions)
	r.db.Model(&models.AppVersion{}).Where("platform = ?", models.AppPlatformIOS).Count(&stats.IOSVersions)

	// Total downloads
	var totalDownloads int64
	r.db.Model(&models.AppVersion{}).Select("COALESCE(SUM(download_count), 0)").Scan(&totalDownloads)
	stats.TotalDownloads = totalDownloads

	// Platform-specific downloads
	var windowsDownloads, androidDownloads, iosDownloads int64
	r.db.Model(&models.AppVersion{}).Where("platform = ?", models.AppPlatformWindows).Select("COALESCE(SUM(download_count), 0)").Scan(&windowsDownloads)
	r.db.Model(&models.AppVersion{}).Where("platform = ?", models.AppPlatformAndroid).Select("COALESCE(SUM(download_count), 0)").Scan(&androidDownloads)
	r.db.Model(&models.AppVersion{}).Where("platform = ?", models.AppPlatformIOS).Select("COALESCE(SUM(download_count), 0)").Scan(&iosDownloads)

	stats.WindowsDownloads = windowsDownloads
	stats.AndroidDownloads = androidDownloads
	stats.IOSDownloads = iosDownloads

	// Total storage used (file_size sum)
	var totalStorage int64
	r.db.Model(&models.AppVersion{}).Select("COALESCE(SUM(file_size), 0)").Scan(&totalStorage)
	stats.TotalStorageUsed = totalStorage

	return stats, nil
}

// DeactivateOtherVersions deactivates all versions for a platform except the specified one
func (r *appVersionRepository) DeactivateOtherVersions(platform models.AppPlatform, exceptID uint) error {
	return r.db.Model(&models.AppVersion{}).
		Where("platform = ? AND id != ?", platform, exceptID).
		Update("is_active", false).Error
}

// IncrementDownloadCount increments the download count for a version
func (r *appVersionRepository) IncrementDownloadCount(id uint) error {
	return r.db.Model(&models.AppVersion{}).
		Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + ?", 1)).Error
}

// GetAllByPlatform retrieves all versions for a specific platform
func (r *appVersionRepository) GetAllByPlatform(platform models.AppPlatform) ([]*models.AppVersion, error) {
	var versions []*models.AppVersion
	err := r.db.
		Where("platform = ?", platform).
		Order("release_date DESC").
		Find(&versions).Error
	if err != nil {
		return nil, err
	}
	return versions, nil
}
