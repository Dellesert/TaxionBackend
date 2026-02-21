-- Migration: Add mute support for chat notifications
-- Per-chat mute via muted_until column on chat_members
-- Global mute via user_mute_preferences table

-- Add muted_until column to chat_members table
ALTER TABLE chat_members
ADD COLUMN IF NOT EXISTS muted_until TIMESTAMPTZ DEFAULT NULL;

-- Index for efficient mute checking during notification sending
CREATE INDEX IF NOT EXISTS idx_chat_members_muted
ON chat_members(user_id, chat_id)
WHERE muted_until IS NOT NULL;

COMMENT ON COLUMN chat_members.muted_until IS 'Timestamp until which notifications are muted. NULL = not muted, far-future = muted forever';

-- Global mute preferences table
CREATE TABLE IF NOT EXISTS user_mute_preferences (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE,
    mute_all_channels_until TIMESTAMPTZ DEFAULT NULL,
    mute_all_groups_until TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_mute_preferences_user_id
ON user_mute_preferences(user_id);

-- Add trigger for auto-updating updated_at
CREATE TRIGGER update_user_mute_preferences_updated_at
    BEFORE UPDATE ON user_mute_preferences
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE user_mute_preferences IS 'Global mute preferences: mute all channels and/or all groups';
COMMENT ON COLUMN user_mute_preferences.mute_all_channels_until IS 'Mute all channel notifications until this time. NULL = not muted';
COMMENT ON COLUMN user_mute_preferences.mute_all_groups_until IS 'Mute all group notifications until this time. NULL = not muted';
