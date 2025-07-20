-- Initial migration for chat service
-- File: services/chat/migrations/001_initial_chat_schema.sql

-- Create chats table
CREATE TABLE IF NOT EXISTS chats (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL DEFAULT '',
    description VARCHAR(500) NOT NULL DEFAULT '',
    type VARCHAR(20) NOT NULL DEFAULT 'private',
    creator_id INTEGER NOT NULL,
    avatar VARCHAR(500) NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_message_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for chats
CREATE INDEX IF NOT EXISTS idx_chats_creator_id ON chats(creator_id);
CREATE INDEX IF NOT EXISTS idx_chats_type ON chats(type);
CREATE INDEX IF NOT EXISTS idx_chats_is_active ON chats(is_active);
CREATE INDEX IF NOT EXISTS idx_chats_last_message_at ON chats(last_message_at);
CREATE INDEX IF NOT EXISTS idx_chats_deleted_at ON chats(deleted_at);

-- Add constraints for enum-like fields
ALTER TABLE chats ADD CONSTRAINT chk_chats_type 
    CHECK (type IN ('private', 'group', 'channel'));

-- Create chat_members table
CREATE TABLE IF NOT EXISTS chat_members (
    id SERIAL PRIMARY KEY,
    chat_id INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    left_at TIMESTAMP NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for chat_members
CREATE INDEX IF NOT EXISTS idx_chat_members_chat_id ON chat_members(chat_id);
CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_members_is_active ON chat_members(is_active);
CREATE INDEX IF NOT EXISTS idx_chat_members_role ON chat_members(role);
CREATE INDEX IF NOT EXISTS idx_chat_members_deleted_at ON chat_members(deleted_at);

-- Add unique constraint to prevent duplicate memberships
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_members_chat_user_unique 
    ON chat_members(chat_id, user_id) WHERE deleted_at IS NULL;

-- Add constraints for enum-like fields
ALTER TABLE chat_members ADD CONSTRAINT chk_chat_members_role 
    CHECK (role IN ('owner', 'admin', 'member'));

-- Create messages table
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    chat_id INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'text',
    status VARCHAR(20) NOT NULL DEFAULT 'sent',
    reply_to_id INTEGER NULL REFERENCES messages(id) ON DELETE SET NULL,
    edited_at TIMESTAMP NULL,
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- File-related fields
    file_name VARCHAR(255) NOT NULL DEFAULT '',
    file_size BIGINT NOT NULL DEFAULT 0,
    file_url VARCHAR(500) NOT NULL DEFAULT '',
    thumbnail_url VARCHAR(500) NOT NULL DEFAULT '',
    mime_type VARCHAR(100) NOT NULL DEFAULT '',
    
    -- Location-related fields
    latitude DECIMAL(10, 8) NULL,
    longitude DECIMAL(11, 8) NULL,
    
    -- System message metadata
    system_data TEXT NOT NULL DEFAULT '',
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for messages
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_reply_to_id ON messages(reply_to_id);
CREATE INDEX IF NOT EXISTS idx_messages_type ON messages(type);
CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_is_deleted ON messages(is_deleted);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_deleted_at ON messages(deleted_at);

-- Composite index for chat messages ordered by time
CREATE INDEX IF NOT EXISTS idx_messages_chat_created_desc 
    ON messages(chat_id, created_at DESC) WHERE is_deleted = FALSE;

-- Add constraints for enum-like fields
ALTER TABLE messages ADD CONSTRAINT chk_messages_type 
    CHECK (type IN ('text', 'image', 'file', 'video', 'audio', 'location', 'system'));

ALTER TABLE messages ADD CONSTRAINT chk_messages_status 
    CHECK (status IN ('sent', 'delivered', 'read', 'failed'));

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_chats_updated_at BEFORE UPDATE ON chats
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_chat_members_updated_at BEFORE UPDATE ON chat_members
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_messages_updated_at BEFORE UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();