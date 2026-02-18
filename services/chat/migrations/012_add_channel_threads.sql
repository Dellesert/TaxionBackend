-- Migration: Add thread support for channel comments
-- thread_root_id = NULL means this is a root-level message (post in channel or regular message)
-- thread_root_id = <message_id> means this is a comment in that message's thread

ALTER TABLE messages ADD COLUMN IF NOT EXISTS thread_root_id BIGINT;

-- Foreign key to messages table
ALTER TABLE messages
  ADD CONSTRAINT fk_messages_thread_root
  FOREIGN KEY (thread_root_id)
  REFERENCES messages(id)
  ON DELETE CASCADE;

-- Index for efficiently fetching thread comments
CREATE INDEX IF NOT EXISTS idx_messages_thread_root_id
  ON messages(thread_root_id) WHERE thread_root_id IS NOT NULL;

-- Composite index for channel main feed queries (thread_root_id IS NULL)
CREATE INDEX IF NOT EXISTS idx_messages_chat_thread_root
  ON messages(chat_id, thread_root_id);

-- Denormalized reply count on root messages for efficient display
ALTER TABLE messages ADD COLUMN IF NOT EXISTS thread_reply_count INT NOT NULL DEFAULT 0;

-- Last reply timestamp for sorting threads by activity
ALTER TABLE messages ADD COLUMN IF NOT EXISTS thread_last_reply_at TIMESTAMP;
