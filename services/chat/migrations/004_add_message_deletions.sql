-- Add message deletions functionality (personal deletions - "delete for me")
-- File: services/chat/migrations/004_add_message_deletions.sql

-- Create message_deletions table
CREATE TABLE IF NOT EXISTS message_deletions (
    id SERIAL PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    deleted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for message_deletions
CREATE INDEX IF NOT EXISTS idx_message_deletions_message_id ON message_deletions(message_id);
CREATE INDEX IF NOT EXISTS idx_message_deletions_user_id ON message_deletions(user_id);
CREATE INDEX IF NOT EXISTS idx_message_deletions_deleted_at ON message_deletions(deleted_at);

-- Create unique constraint to prevent duplicate deletions
CREATE UNIQUE INDEX IF NOT EXISTS idx_message_deletions_unique
    ON message_deletions(message_id, user_id);

-- Create composite index for common queries (filtering messages by user)
CREATE INDEX IF NOT EXISTS idx_message_deletions_user_message
    ON message_deletions(user_id, message_id);

-- Create trigger for updated_at
DROP TRIGGER IF EXISTS update_message_deletions_updated_at ON message_deletions;
CREATE TRIGGER update_message_deletions_updated_at BEFORE UPDATE ON message_deletions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add columns to messages table for global deletions ("delete for everyone")
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_by INTEGER NULL;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP NULL;

-- Create index for deleted_at (for filtering deleted messages)
CREATE INDEX IF NOT EXISTS idx_messages_deleted_at ON messages(deleted_at);

-- Create index for deleted_by (for audit trail)
CREATE INDEX IF NOT EXISTS idx_messages_deleted_by ON messages(deleted_by);

-- Comment on tables and columns
COMMENT ON TABLE message_deletions IS 'Personal message deletions (delete for me) - user-specific visibility';
COMMENT ON COLUMN message_deletions.message_id IS 'Reference to the message';
COMMENT ON COLUMN message_deletions.user_id IS 'User who deleted the message for themselves';
COMMENT ON COLUMN message_deletions.deleted_at IS 'When the message was deleted for this user';

COMMENT ON COLUMN messages.deleted_by IS 'User ID who deleted the message for everyone (null if not deleted globally)';
COMMENT ON COLUMN messages.deleted_at IS 'When the message was deleted for everyone (null if not deleted globally)';
