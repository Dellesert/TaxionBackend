-- Add build_number column to app_versions table
ALTER TABLE app_versions ADD COLUMN build_number INTEGER DEFAULT 0;

-- Update unique constraint to include build_number
DROP INDEX IF EXISTS idx_app_versions_platform_version;
CREATE UNIQUE INDEX idx_app_versions_platform_version_build ON app_versions(platform, version, build_number);

COMMENT ON COLUMN app_versions.build_number IS 'Build number for the version (e.g., 42)';
