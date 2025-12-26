-- Migration: Add indexes for efficient cursor-based pagination
-- These indexes support the new refactored API endpoints for message retrieval

-- Index on (chat_id, id) for cursor-based pagination (before/after queries)
-- This is the most important index for the new pagination system
CREATE INDEX IF NOT EXISTS idx_messages_chat_id_id ON messages(chat_id, id);

-- Index on (chat_id, created_at) for time-based sorting (fallback/alternative)
CREATE INDEX IF NOT EXISTS idx_messages_chat_id_created_at ON messages(chat_id, created_at);

-- Composite index for read receipts query optimization
-- Helps find first unread message efficiently
CREATE INDEX IF NOT EXISTS idx_message_read_receipts_user_message ON message_read_receipts(user_id, message_id);

-- Index for message_deletions to optimize personal deletion queries
CREATE INDEX IF NOT EXISTS idx_message_deletions_user_message ON message_deletions(user_id, message_id);
