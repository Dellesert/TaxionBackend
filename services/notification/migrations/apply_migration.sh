#!/bin/bash
# Script to apply notification service migrations
# Run this script from the TaxionBack root directory

set -e

echo "Applying migration: 001_enable_push_notifications_by_default.sql"
echo ""

# Load DATABASE_URL from .env file if it exists
if [ -f .env ]; then
    export $(grep -v '^#' .env | grep DATABASE_URL | xargs)
fi

if [ -z "$DATABASE_URL" ]; then
    echo "ERROR: DATABASE_URL not found"
    echo "Please set DATABASE_URL environment variable or create .env file"
    exit 1
fi

echo "Using DATABASE_URL from environment"
echo ""

# Check if docker is available and postgres container is running
if command -v docker &> /dev/null; then
    if docker ps | grep -q postgres; then
        echo "Found PostgreSQL container, applying migration via docker exec..."
        docker exec -i postgres psql -U postgres -d tachyon_messenger < services/notification/migrations/001_enable_push_notifications_by_default.sql
        echo ""
        echo "✅ Migration applied successfully!"
        exit 0
    fi
fi

# Try to use psql directly
if command -v psql &> /dev/null; then
    echo "Applying migration using psql..."
    psql "$DATABASE_URL" -f services/notification/migrations/001_enable_push_notifications_by_default.sql
    echo ""
    echo "✅ Migration applied successfully!"
else
    echo ""
    echo "ERROR: psql not found in PATH"
    echo ""
    echo "Please install PostgreSQL client tools or use one of these methods:"
    echo "1. Run Docker and use: docker exec -i postgres_container psql -U postgres -d tachyon_messenger < services/notification/migrations/001_enable_push_notifications_by_default.sql"
    echo "2. Use a PostgreSQL GUI tool like pgAdmin or DBeaver to execute the SQL file"
    echo "3. Install PostgreSQL client tools: apt-get install postgresql-client (Ubuntu/Debian) or brew install postgresql (macOS)"
    exit 1
fi
