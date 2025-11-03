# Tachyon Messenger Backend - Руководство по деплою

## 🚀 Quick Links

- **Development Setup**: См. ниже
- **Configuration**:
  - ⚙️ [Environment Configuration Guide](docs/ENV_CONFIGURATION.md) - Настройка .env файлов
  - 🔄 [Environment Comparison](docs/ENVIRONMENT_COMPARISON.md) - Development vs Production
- **Production Deployment**:
  - 📖 [Quick Start Guide](docs/PRODUCTION_QUICKSTART.md) - Быстрый старт в production (20 минут)
  - 📚 [Full Production Guide](docs/PRODUCTION_DEPLOYMENT.md) - Полная документация по production
  - 🔒 [Security Checklist](docs/PRODUCTION_DEPLOYMENT.md#security-checklist)

---

## Описание проекта

Tachyon Messenger - это микросервисная платформа для обмена сообщениями с поддержкой чата, задач, календаря, опросов и уведомлений. Бекенд построен на Go с использованием фреймворка Gin и развернут через Docker Compose.

**Ключевые возможности:**
- 🔐 **Stateful Session Authentication** - Безопасная аутентификация на основе сессий
- 🔑 **WebAuthn/Passkey Support** - Поддержка биометрической аутентификации
- 💬 **Real-time Chat** - WebSocket для мгновенного обмена сообщениями
- 📋 **Task Management** - Управление задачами и проектами
- 📅 **Calendar** - Календарь событий и встреч
- 📊 **Polls** - Создание и участие в опросах
- 🔔 **Notifications** - Email уведомления
- 📁 **File Storage** - Загрузка и хранение файлов

### Архитектура

Система состоит из следующих микросервисов:

- **Gateway** (порт 8080) - API Gateway для маршрутизации запросов
- **User Service** (порт 8081) - Управление пользователями и аутентификация
- **Chat Service** (порт 8082) - Чат и обмен сообщениями (WebSocket)
- **Task Service** (порт 8083) - Управление задачами
- **Calendar Service** (порт 8084) - Календарь и события
- **Poll Service** (порт 8085) - Опросы и голосования
- **Notification Service** (порт 8087) - Система уведомлений
- **File Service** (порт 8088) - Хранение и управление файлами

### Инфраструктура

- **PostgreSQL 15** (порт 5432) - Основная база данных
- **Redis 7** (порт 6379) - Кэш и хранилище сессий

---

## Системные требования

### Минимальные требования

- **CPU**: 2 ядра
- **RAM**: 4 GB
- **Диск**: 10 GB свободного места
- **ОС**: Linux (Ubuntu 20.04+, Debian 11+, CentOS 8+), macOS 10.15+, Windows 10+ с WSL2

### Рекомендуемые требования для продакшена

- **CPU**: 4+ ядра
- **RAM**: 8+ GB
- **Диск**: 50+ GB SSD
- **ОС**: Linux (Ubuntu 22.04 LTS)

### Необходимое ПО

1. **Docker** версии 20.10.0 или новее
   ```bash
   docker --version
   ```

2. **Docker Compose** версии 2.0.0 или новее
   ```bash
   docker-compose --version
   ```

3. **Git** для клонирования репозитория
   ```bash
   git --version
   ```

---

## Установка Docker

### Ubuntu/Debian

```bash
# Обновление системы
sudo apt-get update
sudo apt-get upgrade -y

# Установка зависимостей
sudo apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

# Добавление официального GPG ключа Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# Добавление репозитория Docker
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Установка Docker Engine
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Добавление пользователя в группу docker
sudo usermod -aG docker $USER

# Перезапуск сессии для применения изменений
newgrp docker
```

### Windows

1. Скачайте [Docker Desktop для Windows](https://www.docker.com/products/docker-desktop)
2. Установите и запустите Docker Desktop
3. Убедитесь, что включен WSL2 backend

### macOS

1. Скачайте [Docker Desktop для macOS](https://www.docker.com/products/docker-desktop)
2. Установите и запустите Docker Desktop

---

## Быстрый старт (Development)

### 1. Клонирование репозитория

```bash
git clone https://github.com/mishkajackson/Taxion
cd TaxionBack
```

### 2. Настройка переменных окружения

Скопируйте файл с примером и настройте под свои нужды:

```bash
cp .env.example .env
```

**Важно!** Отредактируйте файл `.env` и измените следующие параметры:

```env
# Пароли для баз данных
POSTGRES_PASSWORD=надежный_пароль_postgres
REDIS_PASSWORD=надежный_пароль_redis

# Режим аутентификации (session или jwt)
AUTH_MODE=session
SESSION_DURATION_HOURS=0.5

# CORS настройки для фронтенда
CORS_ORIGINS=http://localhost:3000,http://localhost:8080,http://localhost:8093,http://localhost:5173,http://localhost:5174,http://localhost:5175

# Настройки SMTP для отправки email (опционально)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM_EMAIL=noreply@yourdomain.com
```

### 3. Запуск сервисов

```bash
# Сборка образов
docker-compose build --parallel

# Запуск всех сервисов
docker-compose up -d

# Проверка статуса
docker-compose ps
```

### 4. Проверка работоспособности

После запуска подождите 30-60 секунд для инициализации всех сервисов.

```bash
# Проверка здоровья Gateway
curl http://localhost:8080/health

# Проверка всех сервисов через Gateway
curl http://localhost:8080/health/services
```

Ожидаемый ответ:
```json
{
  "status": "healthy",
  "timestamp": "2024-10-29T12:00:00Z",
  "version": "1.0.0"
}
```

---

## Деплой на продакшн

### 1. Подготовка сервера

```bash
# Обновление системы
sudo apt-get update && sudo apt-get upgrade -y

# Установка необходимых пакетов
sudo apt-get install -y curl wget git ufw

# Настройка файрвола
sudo ufw allow 22/tcp      # SSH
sudo ufw allow 80/tcp      # HTTP
sudo ufw allow 443/tcp     # HTTPS
sudo ufw enable
```

### 2. Создание production .env файла

```bash
# Создайте .env файл с production настройками
nano .env
```

Пример production конфигурации:

```env
# ==============================================
# Production Environment Configuration
# ==============================================

# Environment
ENVIRONMENT=production
GIN_MODE=release
LOG_LEVEL=info
LOG_FORMAT=json

# ==============================================
# Database Configuration
# ==============================================
POSTGRES_DB=tachyon_messenger
POSTGRES_USER=tachyon_user
POSTGRES_PASSWORD=СЛОЖНЫЙ_ПАРОЛЬ_БД_ТУТ
POSTGRES_PORT=5432

DATABASE_URL=postgres://tachyon_user:СЛОЖНЫЙ_ПАРОЛЬ_БД_ТУТ@postgres:5432/tachyon_messenger?sslmode=require

# ==============================================
# Redis Configuration
# ==============================================
REDIS_PASSWORD=СЛОЖНЫЙ_ПАРОЛЬ_REDIS_ТУТ
REDIS_PORT=6379
REDIS_URL=redis://:СЛОЖНЫЙ_ПАРОЛЬ_REDIS_ТУТ@redis:6379

# ==============================================
# Authentication Configuration
# ==============================================
# Режим аутентификации: "session"
AUTH_MODE=session

# Длительность сессии в часах (только для session режима)
# Для супер-администраторов рекомендуется 0.5 часа (30 минут)
SESSION_DURATION_HOURS=0.5

# ==============================================
# Service Ports (внутренние)
# ==============================================
GATEWAY_PORT=8080
USER_SERVICE_PORT=8081
CHAT_SERVICE_PORT=8082
TASK_SERVICE_PORT=8083
CALENDAR_SERVICE_PORT=8084
POLL_SERVICE_PORT=8085
NOTIFICATION_SERVICE_PORT=8087
FILE_SERVICE_PORT=8088

# ==============================================
# Security Settings
# ==============================================
DEBUG=false
ENABLE_CORS=true

# CORS Origins - разрешенные источники для CORS запросов
# Добавьте все домены фронтенд приложений
CORS_ORIGINS=https://yourdomain.com,https://www.yourdomain.com,https://admin.yourdomain.com

# Session
SESSION_TIMEOUT=30

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=100
RATE_LIMIT_BURST=20

# ==============================================
# Database Performance
# ==============================================
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=3600

# ==============================================
# Redis Performance
# ==============================================
REDIS_MAX_IDLE=20
REDIS_MAX_ACTIVE=200

# ==============================================
# File Storage
# ==============================================
UPLOAD_DIR=./uploads
MAX_UPLOAD_SIZE=10485760
BASE_URL=https://yourdomain.com

# ==============================================
# Email Configuration (для уведомлений и 2FA)
# ==============================================
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-specific-password
SMTP_FROM_EMAIL=noreply@yourdomain.com
SMTP_FROM_NAME=Tachyon Messenger
SMTP_USE_TLS=true

# ==============================================
# Notification Settings
# ==============================================
NOTIFICATION_CONCURRENT_WORKERS=10
NOTIFICATION_QUEUE_SIZE=5000
NOTIFICATION_RETRY_ATTEMPTS=3
NOTIFICATION_RETRY_DELAY=60

# ==============================================
# WebSocket Settings
# ==============================================
WEBSOCKET_READ_BUFFER_SIZE=2048
WEBSOCKET_WRITE_BUFFER_SIZE=2048
WEBSOCKET_MAX_MESSAGE_SIZE=1024
WEBSOCKET_PING_PERIOD=54

# ==============================================
# Monitoring & Health Checks
# ==============================================
HEALTH_CHECK_TIMEOUT=10
SERVICE_STARTUP_TIMEOUT=60

# ==============================================
# Feature Flags
# ==============================================
FEATURE_EMAIL_NOTIFICATIONS=true
FEATURE_PUSH_NOTIFICATIONS=false
FEATURE_SMS_NOTIFICATIONS=false
FEATURE_ANALYTICS=true

# Logging
ENABLE_REQUEST_LOGGING=true
ENABLE_SQL_LOGGING=false
```

### 3. Настройка SSL/TLS (с использованием Nginx)

Установите Nginx как reverse proxy:

```bash
sudo apt-get install -y nginx certbot python3-certbot-nginx
```

Создайте конфигурацию Nginx:

```bash
sudo nano /etc/nginx/sites-available/tachyon
```

Пример конфигурации:

```nginx
# WebSocket upgrade mapping (должен быть ПЕРЕД server блоками)
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

upstream tachyon_backend {
    server 127.0.0.1:8080;
    keepalive 64;
}

server {
    listen 80;
    server_name yourdomain.com www.yourdomain.com;

    # Перенаправление HTTP на HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com www.yourdomain.com;

    # SSL сертификаты (будут созданы через certbot)
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # SSL настройки
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;

    # Размер загружаемых файлов
    client_max_body_size 10M;

    # Логи
    access_log /var/log/nginx/tachyon_access.log;
    error_log /var/log/nginx/tachyon_error.log;

    # WebSocket поддержка для /api/v1/ws (chat service) - ДОЛЖЕН БЫТЬ ПЕРЕД location /
    location /ws {
        proxy_pass http://tachyon_backend;
        proxy_http_version 1.1;

        # WebSocket headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Увеличенные таймауты для WebSocket
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;

        # Отключить буферизацию для WebSocket
        proxy_buffering off;
    }

    # Proxy настройки для остальных запросов
    location / {
        proxy_pass http://tachyon_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Таймауты
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://tachyon_backend;
        access_log off;
    }
}
```

Активируйте конфигурацию:

```bash
# Создайте символическую ссылку
sudo ln -s /etc/nginx/sites-available/tachyon /etc/nginx/sites-enabled/

# Удалите дефолтную конфигурацию
sudo rm /etc/nginx/sites-enabled/default

# Проверьте конфигурацию
sudo nginx -t

# Перезапустите Nginx
sudo systemctl restart nginx
```

Получите SSL сертификат:

```bash
sudo certbot --nginx -d yourdomain.com -d www.yourdomain.com
```

### 4. Запуск production сервисов

```bash
# Перейдите в директорию проекта
cd /path/to/TaxionBack

# Соберите образы для production
docker-compose up -d  --build --no-cache --parallel

# Для production
docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d --build

# Проверьте статус
docker-compose ps

# Проверьте логи
docker-compose logs -f --tail=100
```

### 5. Настройка автозапуска

Создайте systemd service для автоматического запуска:

```bash
sudo nano /etc/systemd/system/tachyon.service
```

Содержимое файла:

```ini
[Unit]
Description=Tachyon Messenger Backend
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/path/to/TaxionBack
ExecStart=/usr/bin/docker-compose up -d
ExecStop=/usr/bin/docker-compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
```

Активируйте сервис:

```bash
sudo systemctl daemon-reload
sudo systemctl enable tachyon.service
sudo systemctl start tachyon.service
```

---

## Мониторинг и обслуживание

### Просмотр логов

```bash
# Все сервисы
docker-compose logs -f

# Конкретный сервис
docker-compose logs -f user-service
docker-compose logs -f gateway

# Последние N строк
docker-compose logs --tail=100 gateway
```

### Проверка статуса сервисов

```bash
# Статус контейнеров
docker-compose ps

# Проверка здоровья через API
curl http://localhost:8080/health/services
```

### Подключение к базам данных

```bash
# PostgreSQL
docker-compose exec postgres psql -U tachyon_user -d tachyon_messenger

# Redis
docker-compose exec redis redis-cli -a redis_password
```

### Резервное копирование

#### PostgreSQL

```bash
# Создание бэкапа
docker-compose exec -T postgres pg_dump -U tachyon_user tachyon_messenger > backup_$(date +%Y%m%d_%H%M%S).sql

# Восстановление из бэкапа
docker-compose exec -T postgres psql -U tachyon_user tachyon_messenger < backup_20241029_120000.sql
```

#### Redis

```bash
# Создание снапшота
docker-compose exec redis redis-cli -a redis_password BGSAVE

# Копирование RDB файла
docker cp tachyon-redis:/data/dump.rdb ./redis_backup_$(date +%Y%m%d_%H%M%S).rdb
```

#### Полный бэкап Docker volumes

```bash
# Создание бэкапа всех данных
docker run --rm \
  -v tachyon_postgres_data:/data/postgres \
  -v tachyon_redis_data:/data/redis \
  -v tachyon_file_uploads:/data/uploads \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/tachyon_full_backup_$(date +%Y%m%d_%H%M%S).tar.gz /data
```

### Обновление сервисов

```bash
# Остановка сервисов
docker-compose down

# Получение новых изменений из git
git pull

# Пересборка образов
docker-compose build --no-cache --parallel

# Запуск обновленных сервисов
docker-compose up -d

# Проверка статуса
docker-compose ps

# Проверка работоспособности
curl http://localhost:8080/health/services
```

### Масштабирование

Для масштабирования конкретных сервисов:

```bash
# Запуск нескольких инстансов
docker-compose up -d --scale user-service=3 --scale chat-service=2

# Примечание: требуется балансировщик нагрузки (например, Nginx upstream)
```

---

## Устранение неполадок

### Сервисы не запускаются

```bash
# Проверьте логи
docker-compose logs

# Проверьте, не заняты ли порты
sudo netstat -tulpn | grep -E ':(8080|8081|8082|5432|6379)'

# Пересоздайте контейнеры
docker-compose down
docker-compose up -d --force-recreate
```

### База данных недоступна

```bash
# Проверьте статус PostgreSQL
docker-compose ps postgres

# Проверьте логи
docker-compose logs postgres

# Проверьте подключение
docker-compose exec postgres pg_isready -U tachyon_user
```

### Ошибки подключения между сервисами

```bash
# Проверьте Docker сеть
docker network ls
docker network inspect tachyon-network

# Проверьте переменные окружения
docker-compose config

# Пересоздайте сеть
docker-compose down
docker network prune
docker-compose up -d
```

### Проблемы с 2FA

```bash
# Проверьте настройки SMTP
docker-compose exec user-service printenv | grep SMTP

# Проверьте логи отправки email
docker-compose logs user-service | grep -i "2FA\|email"

# Проверьте статус 2FA для пользователя
docker-compose exec postgres psql -U tachyon_user -d tachyon_messenger \
  -c "SELECT id, email, two_factor_enabled FROM users WHERE email='user@example.com';"
```

### Проблемы с сессиями

```bash
# Проверьте режим аутентификации
docker-compose exec user-service printenv | grep AUTH_MODE

# Проверьте Redis подключение
docker-compose exec redis redis-cli -a redis_password ping

# Проверьте активные сессии
docker-compose exec redis redis-cli -a redis_password KEYS "session:*"
```

### Проблемы с производительностью

```bash
# Проверьте использование ресурсов
docker stats

# Очистите неиспользуемые ресурсы
docker system prune -a

# Проверьте размер логов
docker-compose logs --timestamps | wc -l
```

### Полная очистка и переустановка

```bash
# ВНИМАНИЕ: Удалит все данные!
docker-compose down -v
docker system prune -a --volumes -f
rm -rf logs/*

# Запустите заново
docker-compose build --parallel
docker-compose up -d
```

---

## Лицензия

[ Лицензию ]

---

