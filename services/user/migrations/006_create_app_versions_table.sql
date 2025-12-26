-- Create app_versions table for managing application versions across platforms
-- Supports Windows (direct download), Android (APK), and iOS (App Store link)

CREATE TABLE IF NOT EXISTS app_versions (
    id SERIAL PRIMARY KEY,
    platform VARCHAR(20) NOT NULL,
    version VARCHAR(50) NOT NULL,
    changelog TEXT,
    is_critical BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    download_count BIGINT DEFAULT 0,
    file_path VARCHAR(500),
    file_size BIGINT DEFAULT 0,
    checksum VARCHAR(64),
    store_url VARCHAR(500),
    release_date TIMESTAMP NOT NULL,
    uploaded_by_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_app_versions_platform ON app_versions(platform);
CREATE INDEX idx_app_versions_version ON app_versions(version);
CREATE INDEX idx_app_versions_is_critical ON app_versions(is_critical);
CREATE INDEX idx_app_versions_is_active ON app_versions(is_active);
CREATE INDEX idx_app_versions_release_date ON app_versions(release_date);
CREATE INDEX idx_app_versions_uploaded_by_id ON app_versions(uploaded_by_id);
CREATE INDEX idx_app_versions_platform_active ON app_versions(platform, is_active);

-- Add unique constraint to prevent duplicate platform/version combinations
CREATE UNIQUE INDEX idx_app_versions_platform_version ON app_versions(platform, version);

-- Add comments
COMMENT ON TABLE app_versions IS 'Application versions for Windows, Android, and iOS platforms';
COMMENT ON COLUMN app_versions.platform IS 'windows, android, or ios';
COMMENT ON COLUMN app_versions.is_critical IS 'If true, forces users to update';
COMMENT ON COLUMN app_versions.is_active IS 'Only one active version per platform should exist';
COMMENT ON COLUMN app_versions.file_path IS 'File system path for Windows/Android binaries';
COMMENT ON COLUMN app_versions.store_url IS 'App Store URL for iOS';
COMMENT ON COLUMN app_versions.checksum IS 'SHA-256 checksum for file integrity verification';
