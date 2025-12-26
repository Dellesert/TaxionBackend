-- Migration: Enable push notifications by default for all users
-- Date: 2025-12-23
-- Description: Updates all existing user_notification_preferences to enable push notifications
-- This migration is idempotent - it will only run once

-- Create migrations tracking table if not exists
CREATE TABLE IF NOT EXISTS notification_data_migrations (
    id SERIAL PRIMARY KEY,
    migration_name VARCHAR(255) UNIQUE NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Check if migration already applied and execute only if not
DO $$
DECLARE
    migration_exists INTEGER;
    updated_count INTEGER;
BEGIN
    -- Check if migration already applied
    SELECT COUNT(*) INTO migration_exists
    FROM notification_data_migrations
    WHERE migration_name = '001_enable_push_notifications_by_default';

    IF migration_exists = 0 THEN
        -- Apply migration
        UPDATE user_notification_preferences
        SET push_enabled = true
        WHERE push_enabled = false;

        GET DIAGNOSTICS updated_count = ROW_COUNT;

        -- Record migration as applied
        INSERT INTO notification_data_migrations (migration_name)
        VALUES ('001_enable_push_notifications_by_default');

        RAISE NOTICE 'Migration completed: Enabled push notifications for % preference records', updated_count;
    ELSE
        RAISE NOTICE 'Migration already applied, skipping';
    END IF;
END $$;
