-- Migration to fix integer to bigint conversion for chat_id and related fields
-- File: services/chat/migrations/008_fix_bigint_types.sql

-- Step 1: Drop the view that depends on chat_members
DROP VIEW IF EXISTS unread_message_counts;

-- Step 2: Alter column types to bigint
-- Note: We need to drop and recreate foreign key constraints

-- Drop foreign key constraints first
ALTER TABLE chat_members DROP CONSTRAINT IF EXISTS chat_members_chat_id_fkey;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_chat_id_fkey;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_reply_to_id_fkey;
ALTER TABLE message_read_receipts DROP CONSTRAINT IF EXISTS message_read_receipts_message_id_fkey;
ALTER TABLE message_deletions DROP CONSTRAINT IF EXISTS message_deletions_message_id_fkey;

-- Alter all ID columns to bigint
-- Chats table
ALTER TABLE chats ALTER COLUMN id TYPE BIGINT;
ALTER TABLE chats ALTER COLUMN creator_id TYPE BIGINT;
ALTER TABLE chats ALTER COLUMN task_id TYPE BIGINT;

-- Chat members table
ALTER TABLE chat_members ALTER COLUMN id TYPE BIGINT;
ALTER TABLE chat_members ALTER COLUMN chat_id TYPE BIGINT;
ALTER TABLE chat_members ALTER COLUMN user_id TYPE BIGINT;

-- Messages table
ALTER TABLE messages ALTER COLUMN id TYPE BIGINT;
ALTER TABLE messages ALTER COLUMN chat_id TYPE BIGINT;
ALTER TABLE messages ALTER COLUMN sender_id TYPE BIGINT;
ALTER TABLE messages ALTER COLUMN reply_to_id TYPE BIGINT;

-- Message read receipts table
ALTER TABLE message_read_receipts ALTER COLUMN id TYPE BIGINT;
ALTER TABLE message_read_receipts ALTER COLUMN message_id TYPE BIGINT;
ALTER TABLE message_read_receipts ALTER COLUMN user_id TYPE BIGINT;

-- Message deletions table (if exists)
ALTER TABLE message_deletions ALTER COLUMN id TYPE BIGINT;
ALTER TABLE message_deletions ALTER COLUMN message_id TYPE BIGINT;
ALTER TABLE message_deletions ALTER COLUMN user_id TYPE BIGINT;

-- Recreate foreign key constraints
ALTER TABLE chat_members
    ADD CONSTRAINT chat_members_chat_id_fkey
    FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE;

ALTER TABLE messages
    ADD CONSTRAINT messages_chat_id_fkey
    FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE;

ALTER TABLE messages
    ADD CONSTRAINT messages_reply_to_id_fkey
    FOREIGN KEY (reply_to_id) REFERENCES messages(id) ON DELETE SET NULL;

ALTER TABLE message_read_receipts
    ADD CONSTRAINT message_read_receipts_message_id_fkey
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE;

ALTER TABLE message_deletions
    ADD CONSTRAINT message_deletions_message_id_fkey
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE;

-- Step 3: Recreate the view with updated types
CREATE VIEW unread_message_counts AS
SELECT
    m.chat_id,
    cm.user_id,
    COUNT(m.id) AS unread_count
FROM messages m
JOIN chat_members cm ON m.chat_id = cm.chat_id
LEFT JOIN message_read_receipts mrr ON m.id = mrr.message_id AND mrr.user_id = cm.user_id
WHERE
    m.sender_id <> cm.user_id
    AND m.is_deleted = false
    AND cm.is_active = true
    AND mrr.id IS NULL
GROUP BY m.chat_id, cm.user_id;
