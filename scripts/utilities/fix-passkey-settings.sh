#!/bin/bash

# Fix system settings to allow passkey login
# This script updates the auth_mode to allow passkey authentication

echo "Updating system settings to allow passkey login..."

docker exec taxionback-postgres-1 psql -U tachyon_user -d tachyon_messenger -c "
UPDATE system_settings
SET auth_mode = 'password_or_passkey',
    allow_passkey_login = true,
    allow_multiple_passkeys = true
WHERE id = 1;
"

echo "Settings updated! Passkey login is now enabled."
echo "Current settings:"

docker exec taxionback-postgres-1 psql -U tachyon_user -d tachyon_messenger -c "
SELECT id, auth_mode, allow_passkey_login, allow_multiple_passkeys
FROM system_settings
LIMIT 1;
"
