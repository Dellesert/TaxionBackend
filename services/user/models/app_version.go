package models

import (
	"time"

	"tachyon-messenger/shared/models"
)

// AppPlatform represents the platform type
type AppPlatform string

const (
	AppPlatformWindows AppPlatform = "windows"
	AppPlatformAndroid AppPlatform = "android"
	AppPlatformIOS     AppPlatform = "ios"
)

// AppVersion represents an application version
type AppVersion struct {
	models.BaseModel
	Platform      AppPlatform `gorm:"not null;index;size:20" json:"platform" validate:"required,oneof=windows android ios"`
	Version       string      `gorm:"not null;size:50;index" json:"version" validate:"required,max=50"`
	BuildNumber   int         `gorm:"default:0" json:"build_number"`
	Changelog     string      `gorm:"type:text" json:"changelog,omitempty"`
	IsCritical    bool        `gorm:"default:false;index" json:"is_critical"`    // Force update flag
	IsActive      bool        `gorm:"default:true;index" json:"is_active"`       // Only one active version per platform
	DownloadCount int64       `gorm:"default:0" json:"download_count"`           // Download statistics
	FilePath      string      `gorm:"size:500" json:"file_path,omitempty"`       // For Windows/Android: path on disk
	FileSize      int64       `gorm:"default:0" json:"file_size"`                // File size in bytes
	Checksum      string      `gorm:"size:64" json:"checksum,omitempty"`         // SHA-256 checksum for integrity
	StoreURL      string      `gorm:"size:500" json:"store_url,omitempty"`       // For iOS: App Store URL
	ReleaseDate   time.Time   `gorm:"not null;index" json:"release_date"`        // Release date
	UploadedByID  uint        `gorm:"not null;index" json:"uploaded_by_id"`      // Admin who uploaded
	UploadedBy    *User       `gorm:"foreignKey:UploadedByID" json:"uploaded_by,omitempty"`
}

// TableName returns the table name for AppVersion model
func (AppVersion) TableName() string {
	return "app_versions"
}

// CreateAppVersionRequest represents request for creating a new app version
type CreateAppVersionRequest struct {
	Platform    string `form:"platform" binding:"required,oneof=windows android ios"`
	Version     string `form:"version" binding:"required,max=50"`
	BuildNumber int    `form:"build_number"`
	Changelog   string `form:"changelog"`
	IsCritical  bool   `form:"is_critical"`
	StoreURL    string `form:"store_url"` // Only for iOS
	// File is handled separately via multipart form
}

// UpdateAppVersionRequest represents request for updating app version metadata
type UpdateAppVersionRequest struct {
	Changelog   *string `json:"changelog,omitempty"`
	IsCritical  *bool   `json:"is_critical,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	StoreURL    *string `json:"store_url,omitempty"`
	BuildNumber *int    `json:"build_number,omitempty"`
}

// AppVersionResponse represents app version response
type AppVersionResponse struct {
	ID            uint        `json:"id"`
	Platform      AppPlatform `json:"platform"`
	Version       string      `json:"version"`
	BuildNumber   int         `json:"build_number"`
	Changelog     string      `json:"changelog,omitempty"`
	IsCritical    bool        `json:"is_critical"`
	IsActive      bool        `json:"is_active"`
	DownloadCount int64       `json:"download_count"`
	FileSize      int64       `json:"file_size"`
	Checksum      string      `json:"checksum,omitempty"`
	StoreURL      string      `json:"store_url,omitempty"`
	ReleaseDate   time.Time   `json:"release_date"`
	UploadedByID  uint        `json:"uploaded_by_id"`
	UploadedBy    *UserResponse `json:"uploaded_by,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// ToResponse converts AppVersion to AppVersionResponse
func (av *AppVersion) ToResponse() *AppVersionResponse {
	response := &AppVersionResponse{
		ID:            av.ID,
		Platform:      av.Platform,
		Version:       av.Version,
		BuildNumber:   av.BuildNumber,
		Changelog:     av.Changelog,
		IsCritical:    av.IsCritical,
		IsActive:      av.IsActive,
		DownloadCount: av.DownloadCount,
		FileSize:      av.FileSize,
		Checksum:      av.Checksum,
		StoreURL:      av.StoreURL,
		ReleaseDate:   av.ReleaseDate,
		UploadedByID:  av.UploadedByID,
		CreatedAt:     av.CreatedAt,
		UpdatedAt:     av.UpdatedAt,
	}

	// Include uploader if loaded
	if av.UploadedBy != nil {
		response.UploadedBy = av.UploadedBy.ToResponse()
	}

	return response
}

// AppVersionListResponse represents paginated list of app versions
type AppVersionListResponse struct {
	Versions   []*AppVersionResponse `json:"versions"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	TotalPages int                   `json:"total_pages"`
}

// LatestVersionsResponse represents latest versions for all platforms
type LatestVersionsResponse struct {
	Windows *AppVersionResponse `json:"windows,omitempty"`
	Android *AppVersionResponse `json:"android,omitempty"`
	IOS     *AppVersionResponse `json:"ios,omitempty"`
}

// AppVersionStatsResponse represents app version statistics
type AppVersionStatsResponse struct {
	TotalVersions      int64 `json:"total_versions"`
	WindowsVersions    int64 `json:"windows_versions"`
	AndroidVersions    int64 `json:"android_versions"`
	IOSVersions        int64 `json:"ios_versions"`
	TotalDownloads     int64 `json:"total_downloads"`
	WindowsDownloads   int64 `json:"windows_downloads"`
	AndroidDownloads   int64 `json:"android_downloads"`
	IOSDownloads       int64 `json:"ios_downloads"`
	TotalStorageUsed   int64 `json:"total_storage_used"` // Bytes
}
