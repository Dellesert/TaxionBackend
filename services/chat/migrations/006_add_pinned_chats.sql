-- Add is_pinned column to chat_members table
ALTER TABLE chat_members
ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT false;

-- Create index for faster queries on pinned chats
CREATE INDEX IF NOT EXISTS idx_chat_members_pinned
ON chat_members(user_id, is_pinned)
WHERE is_pinned = true;

-- Add comment to column
COMMENT ON COLUMN chat_members.is_pinned IS 'Indicates if the user has pinned this chat to the top of the list';
