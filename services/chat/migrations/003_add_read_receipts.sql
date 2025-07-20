-- Add message read receipts functionality
-- File: services/chat/migrations/003_add_read_receipts.sql

-- Create message_read_receipts table
CREATE TABLE IF NOT EXISTS message_read_receipts (
    id SERIAL PRIMARY KEY,
    message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    read_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for message_read_receipts
CREATE INDEX IF NOT EXISTS idx_message_read_receipts_message_id ON message_read_receipts(message_id);
CREATE INDEX IF NOT EXISTS idx_message_read_receipts_user_id ON message_read_receipts(user_id);
CREATE INDEX IF NOT EXISTS idx_message_read_receipts_read_at ON message_read_receipts(read_at);
CREATE INDEX IF NOT EXISTS idx_message_read_receipts_deleted_at ON message_read_receipts(deleted_at);

-- Create unique constraint to prevent duplicate read receipts
CREATE UNIQUE INDEX IF NOT EXISTS idx_message_read_receipts_unique 
    ON message_read_receipts(message_id, user_id) WHERE deleted_at IS NULL;

-- Create trigger for updated_at
CREATE TRIGGER update_message_read_receipts_updated_at BEFORE UPDATE ON message_read_receipts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_unread_messages_by_chat_user 
    ON message_read_receipts(user_id, message_id);

-- Add view for unread message counts per chat per user
CREATE OR REPLACE VIEW unread_message_counts AS
SELECT 
    m.chat_id,
    cm.user_id,
    COUNT(m.id) as unread_count
FROM messages m
JOIN chat_members cm ON m.chat_id = cm.chat_id
LEFT JOIN message_read_receipts mrr ON m.id = mrr.message_id AND mrr.user_id = cm.user_id
WHERE 
    m.sender_id != cm.user_id  -- Don't count own messages
    AND m.is_deleted = FALSE
    AND cm.is_active = TRUE
    AND mrr.id IS NULL  -- No read receipt = unread
GROUP BY m.chat_id, cm.user_id;