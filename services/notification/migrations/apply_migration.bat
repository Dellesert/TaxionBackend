@echo off
REM Script to apply notification service migrations
REM Run this script from the TaxionBack root directory

echo Applying migration: 001_enable_push_notifications_by_default.sql
echo.

REM Try to get DATABASE_URL from .env file
for /f "tokens=1,2 delims==" %%a in ('findstr /r "^DATABASE_URL" .env 2^>nul') do set DATABASE_URL=%%b

if "%DATABASE_URL%"=="" (
    echo ERROR: DATABASE_URL not found in .env file
    echo Please set DATABASE_URL environment variable or update .env file
    pause
    exit /b 1
)

echo Using DATABASE_URL from .env file
echo.

REM Check if docker is available and postgres container is running
docker ps >nul 2>&1
if %errorlevel% equ 0 (
    echo Checking for PostgreSQL container...
    docker ps | findstr postgres >nul
    if %errorlevel% equ 0 (
        echo Found PostgreSQL container, applying migration via docker exec...
        docker exec -i postgres psql -U postgres -d tachyon_messenger < services\notification\migrations\001_enable_push_notifications_by_default.sql
        if %errorlevel% equ 0 (
            echo.
            echo ✅ Migration applied successfully!
        ) else (
            echo.
            echo ❌ Migration failed!
        )
        pause
        exit /b 0
    )
)

REM Try to use psql directly
where psql >nul 2>&1
if %errorlevel% equ 0 (
    echo Applying migration using psql...
    psql "%DATABASE_URL%" -f services\notification\migrations\001_enable_push_notifications_by_default.sql
    if %errorlevel% equ 0 (
        echo.
        echo ✅ Migration applied successfully!
    ) else (
        echo.
        echo ❌ Migration failed!
    )
) else (
    echo.
    echo ERROR: psql not found in PATH
    echo.
    echo Please install PostgreSQL client tools or use one of these methods:
    echo 1. Run Docker and use: docker exec -i postgres_container psql -U postgres -d tachyon_messenger ^< services\notification\migrations\001_enable_push_notifications_by_default.sql
    echo 2. Use a PostgreSQL GUI tool like pgAdmin or DBeaver to execute the SQL file
    echo 3. Install PostgreSQL client tools and run this script again
)

pause
