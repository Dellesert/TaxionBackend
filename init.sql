-- File: init.sql (корень проекта)
-- Database initialization script for Tachyon Messenger

-- Create database if not exists (handled by POSTGRES_DB env var)
-- This script runs automatically when PostgreSQL container starts

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Create custom functions for triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Set timezone
SET timezone = 'UTC';

-- Create schema for application (optional)
-- CREATE SCHEMA IF NOT EXISTS tachyon;

-- Log initialization
DO $$
BEGIN
    RAISE NOTICE 'Tachyon Messenger database initialized successfully';
    RAISE NOTICE 'Database: %, User: %', current_database(), current_user;
    RAISE NOTICE 'Timezone: %', current_setting('timezone');
END $$;