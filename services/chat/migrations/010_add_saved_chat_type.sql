 -- Migration: Add saved chat type support
-- This migration adds support for "saved" chat type (personal favorites/bookmarks chat)
-- Add unique index for saved chats (one per user)
-- This ensures each user can only have one saved chat
CREATE UNIQUE INDEX IF NOT EXISTS idx_chats_saved_per_user
ON chats(creator_id)
WHERE type = 'saved' AND is_active = true;

-- Add index for faster lookup of saved chats by type
CREATE INDEX IF NOT EXISTS idx_chats_type_saved
ON chats(type)
WHERE type = 'saved';

-- Create saved chats for all existing users who don't have one yet
-- Step 1: Insert saved chats for users
INSERT INTO chats (name, description, type, creator_id, is_active, created_at, updated_at)
SELECT
    'Избранное' as name,
    'Сохраненные сообщения' as description,
    'saved' as type,
    u.id as creator_id,
    true as is_active,
    NOW() as created_at,
    NOW() as updated_at
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM chats c
    WHERE c.creator_id = u.id
    AND c.type = 'saved'
    AND c.is_active = true
);

-- Step 2: Add chat members for the newly created saved chats
INSERT INTO chat_members (chat_id, user_id, role, joined_at, is_active, is_favorite, is_pinned, is_hidden, created_at, updated_at)
SELECT
    c.id as chat_id,
    c.creator_id as user_id,
    'owner' as role,
    NOW() as joined_at,
    true as is_active,
    false as is_favorite,
    true as is_pinned,  -- Saved chat is always pinned
    false as is_hidden,
    NOW() as created_at,
    NOW() as updated_at
FROM chats c
WHERE c.type = 'saved'
AND c.is_active = true
AND NOT EXISTS (
    SELECT 1 FROM chat_members cm
    WHERE cm.chat_id = c.id
    AND cm.user_id = c.creator_id
    AND cm.is_active = true
);
