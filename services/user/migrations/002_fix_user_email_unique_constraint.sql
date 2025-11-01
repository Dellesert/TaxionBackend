-- Migration: Fix user email unique constraint to allow reuse after soft delete
-- Date: 2025-11-01
-- Description: Replaces the global unique constraint on email with a partial unique index
--              that only applies to non-deleted users (where deleted_at IS NULL).
--              This allows the same email to be reused for a new user after the previous
--              user with that email has been soft-deleted.

-- Drop the old unique constraint on email if it exists
DROP INDEX IF EXISTS idx_users_email;

-- Create a partial unique index that only applies to non-deleted users
-- This allows the same email to be reused after a user is soft-deleted
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;

-- Verification query (optional, for reference):
-- SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'users' AND indexname = 'idx_users_email';
