#!/bin/bash

# Скрипт для переключения .env между Docker и локальным режимом
# Использование: ./setup-env.sh [docker|local]

ENV_FILE="../../.env"

if [ "$1" == "local" ]; then
    echo "🔧 Настройка .env для локального запуска скриптов..."

    # Заменяем postgres -> localhost
    sed -i 's|postgres://tachyon_user:tachyon_password@postgres:|postgres://tachyon_user:tachyon_password@localhost:|g' "$ENV_FILE"
    sed -i 's|redis://:redis_password@redis:|redis://:redis_password@localhost:|g' "$ENV_FILE"

    echo "✅ .env настроен для локального запуска (localhost)"
    echo ""
    echo "Теперь запустите PostgreSQL и Redis в Docker:"
    echo "  docker compose up -d postgres redis"
    echo ""
    echo "Затем запустите скрипт seeding:"
    echo "  go run scripts/seed/main.go --all --clean"

elif [ "$1" == "docker" ]; then
    echo "🐳 Настройка .env для Docker..."

    # Заменяем localhost -> postgres/redis
    sed -i 's|postgres://tachyon_user:tachyon_password@localhost:|postgres://tachyon_user:tachyon_password@postgres:|g' "$ENV_FILE"
    sed -i 's|redis://:redis_password@localhost:|redis://:redis_password@redis:|g' "$ENV_FILE"

    echo "✅ .env настроен для Docker (postgres/redis)"
    echo ""
    echo "Теперь можно запускать приложение в Docker:"
    echo "  docker compose up"

else
    echo "Использование: $0 [docker|local]"
    echo ""
    echo "  local  - Настроить для локального запуска скриптов (localhost)"
    echo "  docker - Настроить для запуска в Docker (postgres/redis)"
    exit 1
fi
