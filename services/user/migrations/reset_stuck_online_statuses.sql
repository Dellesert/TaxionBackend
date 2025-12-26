-- Migration to reset all stuck online statuses
-- This is a one-time migration to clean up users who are stuck in "online" status
-- Run this migration once to reset all users to offline

UPDATE users
SET
    status = 'offline',
    last_active_at = updated_at,
    updated_at = CURRENT_TIMESTAMP
WHERE status = 'online';
