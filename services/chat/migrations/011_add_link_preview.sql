-- Migration: Add link_preview column to messages table
-- Stores Open Graph metadata for URLs found in message content as JSON text

ALTER TABLE messages ADD COLUMN IF NOT EXISTS link_preview TEXT;
