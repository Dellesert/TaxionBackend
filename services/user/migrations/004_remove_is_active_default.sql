-- Remove default value for is_active column to allow explicit false values
-- This allows creating inactive users who need to activate via invitation

ALTER TABLE users ALTER COLUMN is_active DROP DEFAULT;
