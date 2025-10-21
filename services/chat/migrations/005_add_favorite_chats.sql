-- Migration: Add is_favorite column to chat_members table
-- This allows users to mark chats as favorites

-- Add is_favorite column to chat_members table
ALTER TABLE chat_members
ADD COLUMN IF NOT EXISTS is_favorite BOOLEAN NOT NULL DEFAULT false;

-- Create index for faster queries on favorite chats
CREATE INDEX IF NOT EXISTS idx_chat_members_favorite
ON chat_members(user_id, is_favorite)
WHERE is_favorite = true;

-- Add comment to column
COMMENT ON COLUMN chat_members.is_favorite IS 'Indicates if the user has marked this chat as favorite';
