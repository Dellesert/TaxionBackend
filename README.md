# Tachyon Messenger Backend - Руководство по деплою

## Описание проекта

Tachyon Messenger - это микросервисная платформа для обмена сообщениями с поддержкой чата, задач, календаря, опросов и уведомлений. Бекенд построен на Go с использованием фреймворка Gin и развернут через Docker Compose.

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

### CentOS/RHEL

```bash
# Установка утилит
sudo yum install -y yum-utils

# Добавление репозитория Docker
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

# Установка Docker
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Запуск Docker
sudo systemctl start docker
sudo systemctl enable docker

# Добавление пользователя в группу docker
sudo usermod -aG docker $USER
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
git clone <URL_ВАШЕГО_РЕПОЗИТОРИЯ>
cd TaxionBack
```

### 2. Настройка переменных окружения

Скопируйте файл с примером и настройте под свои нужды:

```bash
cp .env.example .env
```

**Важно!** Отредактируйте файл `.env` и измените следующие параметры:

```env
# ОБЯЗАТЕЛЬНО измените в продакшене!
JWT_SECRET=ваш-супер-секретный-ключ-минимум-32-символа

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
# JWT Configuration
# ==============================================
# ОБЯЗАТЕЛЬНО используйте криптографически стойкий ключ!
JWT_SECRET=ВАШИХ_СУПЕР_СЕКРЕТНЫЙ_КЛЮЧ_МИНИМУМ_64_СИМВОЛА_ДЛЯ_ПРОДАКШЕНА

# Время жизни токенов (в минутах)
JWT_ACCESS_TOKEN_EXPIRE=15
JWT_REFRESH_TOKEN_EXPIRE=10080

# ==============================================
# Authentication Configuration
# ==============================================
# Режим аутентификации: "session" или "jwt"
# session - использует HTTP-only cookies (рекомендуется для веб-приложений)
# jwt - использует JWT токены (рекомендуется для мобильных приложений)
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
upstream tachyon_backend {
    server 127.0.0.1:8080;
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

    # Proxy настройки
    location / {
        proxy_pass http://tachyon_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
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

    # WebSocket поддержка
    location /ws {
        proxy_pass http://tachyon_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Увеличенные таймауты для WebSocket
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
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
docker-compose build --no-cache --parallel

# Запустите сервисы
docker-compose up -d

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

## Новые функции

### 1. Двухфакторная аутентификация (2FA)

Система поддерживает двухфакторную аутентификацию через email для супер-администраторов:

#### Включение 2FA для пользователя

Через админ панель:
- Перейдите в раздел "Пользователи"
- Найдите нужного пользователя
- Включите переключатель в колонке "2FA"

Через API:
```bash
curl -X PUT http://localhost:8080/api/v1/admin/users/{user_id}/2fa \
  -H "Content-Type: application/json" \
  -H "Cookie: session_id=YOUR_SESSION_ID" \
  -d '{"two_factor_enabled": true}'
```

#### Глобальное управление 2FA

Через админ панель в разделе "Безопасность":
- Переключатель для включения/отключения 2FA для всех пользователей
- Статистика использования 2FA

#### Процесс входа с 2FA

1. Пользователь вводит email и пароль
2. Если 2FA включен, на email отправляется 6-значный код
3. Пользователь вводит код из email
4. При успешной верификации создается сессия

#### API Endpoints для 2FA

```bash
# Отправка 2FA кода
POST /api/v1/auth/2fa/send
{
  "email": "user@example.com",
  "password": "userpassword"
}

# Верификация 2FA кода
POST /api/v1/auth/2fa/verify
{
  "email": "user@example.com",
  "code": "123456"
}
```

### 2. Сброс пароля через админ панель

Супер-администраторы могут сбрасывать пароли пользователей:

#### Через админ панель:
- Перейдите в раздел "Пользователи"
- Нажмите кнопку "Сбросить пароль" для нужного пользователя
- Введите новый пароль (минимум 8 символов) или используйте генератор паролей
- Подтвердите изменения

#### Через API:
```bash
curl -X POST http://localhost:8080/api/v1/admin/users/{user_id}/reset-password \
  -H "Content-Type: application/json" \
  -H "Cookie: session_id=YOUR_SESSION_ID" \
  -d '{"new_password": "NewSecurePassword123"}'
```

### 3. Session-based аутентификация

Система поддерживает два режима аутентификации:

#### Session Mode (рекомендуется для веб-приложений)
- Использует HTTP-only cookies
- Более безопасен против XSS атак
- Автоматическая ротация сессий
- Настраиваемое время жизни сессии

Настройка в `.env`:
```env
AUTH_MODE=session
SESSION_DURATION_HOURS=0.5  # 30 минут для супер-админов
```

#### JWT Mode (рекомендуется для мобильных приложений)
- Использует JWT токены в headers
- Подходит для мобильных приложений
- Поддержка refresh токенов

Настройка в `.env`:
```env
AUTH_MODE=jwt
JWT_ACCESS_TOKEN_EXPIRE=15      # 15 минут
JWT_REFRESH_TOKEN_EXPIRE=10080  # 7 дней
```

### 4. Управление отделами

Полный CRUD для управления отделами организации:

#### Через админ панель:
- Создание отделов с указанием руководителя
- Редактирование информации об отделе
- Удаление отделов (с проверкой на наличие сотрудников)
- Просмотр списка сотрудников отдела

#### API Endpoints:
```bash
# Получить список отделов
GET /api/v1/admin/departments

# Создать отдел
POST /api/v1/admin/departments
{
  "name": "IT Department",
  "description": "Information Technology",
  "head_id": 5
}

# Обновить отдел
PUT /api/v1/admin/departments/{id}
{
  "name": "IT Department Updated",
  "description": "Updated description"
}

# Удалить отдел
DELETE /api/v1/admin/departments/{id}
```

### 5. Расширенная страница безопасности

Централизованное управление настройками безопасности:

#### Текущие функции:
- Глобальное управление 2FA
- Статистика использования 2FA
- Модульная архитектура для добавления новых функций

#### Планируемые функции:
- Passkey / WebAuthn аутентификация
- IP Whitelist
- Логи безопасности
- Управление сессиями

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

## API Endpoints

После запуска доступны следующие основные эндпоинты через Gateway (http://localhost:8080):

### Health & Status
- `GET /health` - Статус Gateway
- `GET /health/services` - Статус всех микросервисов

### Authentication
- `POST /api/v1/auth/login` - Вход (обычный пользователь)
- `POST /api/v1/auth/login/superadmin` - Вход (супер-администратор)
- `POST /api/v1/auth/2fa/send` - Отправка 2FA кода
- `POST /api/v1/auth/2fa/verify` - Верификация 2FA кода
- `POST /api/v1/auth/logout` - Выход
- `POST /api/v1/auth/refresh` - Обновление токенов (JWT режим)

### User Service
- `POST /api/users/register` - Регистрация
- `GET /api/users/me` - Профиль пользователя
- `PUT /api/users/me` - Обновление профиля

### Admin - User Management
- `GET /api/v1/admin/users` - Список пользователей
- `POST /api/v1/admin/users` - Создать пользователя
- `PUT /api/v1/admin/users/:id` - Обновить пользователя
- `DELETE /api/v1/admin/users/:id` - Удалить пользователя
- `PUT /api/v1/admin/users/:id/2fa` - Управление 2FA пользователя
- `POST /api/v1/admin/users/:id/reset-password` - Сброс пароля пользователя
- `PUT /api/v1/admin/users/:id/activate` - Активировать пользователя
- `PUT /api/v1/admin/users/:id/deactivate` - Деактивировать пользователя

### Admin - Department Management
- `GET /api/v1/admin/departments` - Список отделов
- `POST /api/v1/admin/departments` - Создать отдел
- `GET /api/v1/admin/departments/:id` - Получить отдел
- `PUT /api/v1/admin/departments/:id` - Обновить отдел
- `DELETE /api/v1/admin/departments/:id` - Удалить отдел
- `GET /api/v1/admin/departments/:id/users` - Пользователи отдела

### Chat Service
- `GET /api/chats` - Список чатов
- `POST /api/chats` - Создать чат
- `GET /api/chats/:id/messages` - История сообщений
- `POST /api/chats/:id/messages` - Отправить сообщение
- `WS /api/ws/chat/:id` - WebSocket соединение

### Task Service
- `GET /api/tasks` - Список задач
- `POST /api/tasks` - Создать задачу
- `GET /api/tasks/:id` - Детали задачи
- `PUT /api/tasks/:id` - Обновить задачу
- `DELETE /api/tasks/:id` - Удалить задачу

### Calendar Service
- `GET /api/events` - Список событий
- `POST /api/events` - Создать событие
- `GET /api/events/:id` - Детали события
- `PUT /api/events/:id` - Обновить событие
- `DELETE /api/events/:id` - Удалить событие

### Poll Service
- `GET /api/polls` - Список опросов
- `POST /api/polls` - Создать опрос
- `GET /api/polls/:id` - Детали опроса
- `POST /api/polls/:id/vote` - Проголосовать

### File Service
- `POST /api/files/upload` - Загрузить файл
- `GET /api/files/:id` - Скачать файл
- `DELETE /api/files/:id` - Удалить файл

### Notification Service
- `GET /api/notifications` - Список уведомлений
- `PUT /api/notifications/:id/read` - Отметить как прочитанное

---

## Безопасность

### Рекомендации для продакшена

1. **Измените все пароли и секретные ключи**
   - JWT_SECRET (минимум 64 символа)
   - POSTGRES_PASSWORD
   - REDIS_PASSWORD

2. **Настройте 2FA**
   - Включите обязательную 2FA для всех администраторов
   - Настройте SMTP для отправки кодов
   - Регулярно проверяйте логи 2FA

3. **Используйте session-based аутентификацию для веб-приложений**
   - Установите `AUTH_MODE=session`
   - Настройте короткое время жизни сессии для администраторов (30 минут)
   - Используйте HTTPS для защиты cookies

4. **Используйте HTTPS**
   - Настройте SSL сертификаты через Let's Encrypt
   - Перенаправляйте HTTP на HTTPS
   - Используйте HSTS заголовки

5. **Настройте файрвол**
   - Закройте прямой доступ к портам сервисов
   - Открывайте только 80, 443 и SSH (22)
   - Используйте fail2ban для защиты от брутфорса

6. **Настройте CORS правильно**
   - Укажите только разрешенные домены в `CORS_ORIGINS`
   - Не используйте `*` в продакшене
   - Разделяйте домены запятыми

7. **Регулярные обновления**
   - Обновляйте Docker образы
   - Применяйте security патчи ОС
   - Следите за уязвимостями в зависимостях

8. **Мониторинг и логирование**
   - Настройте централизованное логирование
   - Используйте инструменты мониторинга (Prometheus, Grafana)
   - Настройте алерты для критичных событий
   - Регулярно проверяйте логи безопасности

9. **Резервное копирование**
   - Автоматизируйте бэкапы баз данных
   - Храните бэкапы в безопасном месте
   - Регулярно тестируйте восстановление
   - Шифруйте бэкапы

10. **Rate Limiting**
    - Настроен в переменных окружения
    - Дополнительно можно настроить на уровне Nginx
    - Мониторьте подозрительную активность

11. **Управление паролями**
    - Используйте сильные пароли (минимум 8 символов)
    - Регулярно меняйте пароли администраторов
    - Используйте функцию сброса паролей вместо прямого доступа к БД

---

## Переменные окружения

### Обязательные переменные

```env
# Database
POSTGRES_PASSWORD=      # Пароль PostgreSQL
REDIS_PASSWORD=        # Пароль Redis

# Security
JWT_SECRET=            # Секретный ключ для JWT (минимум 64 символа)
AUTH_MODE=             # Режим аутентификации: session или jwt
```

### Аутентификация

```env
# Режим аутентификации
AUTH_MODE=session                    # session или jwt
SESSION_DURATION_HOURS=0.5          # Длительность сессии в часах (для session режима)

# JWT настройки (для jwt режима)
JWT_ACCESS_TOKEN_EXPIRE=15          # Минуты
JWT_REFRESH_TOKEN_EXPIRE=10080      # Минуты (7 дней)
```

### CORS

```env
# Разрешенные источники для CORS запросов (через запятую)
CORS_ORIGINS=http://localhost:3000,http://localhost:5173,https://yourdomain.com
```

### Email (для 2FA и уведомлений)

```env
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM_EMAIL=noreply@yourdomain.com
SMTP_FROM_NAME=Tachyon Messenger
SMTP_USE_TLS=true
```

### База данных

```env
POSTGRES_DB=tachyon_messenger
POSTGRES_USER=tachyon_user
POSTGRES_PASSWORD=secure_password
POSTGRES_PORT=5432
DATABASE_URL=postgres://tachyon_user:secure_password@postgres:5432/tachyon_messenger
```

### Redis

```env
REDIS_PASSWORD=secure_password
REDIS_PORT=6379
REDIS_URL=redis://:secure_password@redis:6379
```

### Производительность

```env
# Database
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=3600

# Redis
REDIS_MAX_IDLE=20
REDIS_MAX_ACTIVE=200
```

### Rate Limiting

```env
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=100
RATE_LIMIT_BURST=20
```

### Логирование

```env
ENVIRONMENT=production              # production, development, или staging
GIN_MODE=release                   # release или debug
LOG_LEVEL=info                     # debug, info, warn, error
LOG_FORMAT=json                    # json или text
ENABLE_REQUEST_LOGGING=true
ENABLE_SQL_LOGGING=false           # Только для отладки
```

---

## Поддержка и контакты

При возникновении проблем:

1. Проверьте логи: `docker-compose logs -f`
2. Проверьте статус сервисов: `docker-compose ps`
3. Проверьте [Issues](ссылка_на_github_issues) в репозитории
4. Создайте новый Issue с описанием проблемы

---

## Лицензия

[Укажите лицензию вашего проекта]

---

## Changelog

### Version 1.1.0 (2025-10-31)
- ✨ Добавлена двухфакторная аутентификация (2FA) через email
- ✨ Реализован сброс паролей через админ панель
- ✨ Добавлена поддержка session-based аутентификации
- ✨ Реализовано управление отделами (CRUD)
- ✨ Добавлена расширенная страница безопасности
- ✨ Глобальное управление 2FA для всех пользователей
- ✨ Индивидуальное управление 2FA для каждого пользователя
- 🔒 Улучшена безопасность: проверка 2FA при входе
- 🔒 Session timeout для супер-администраторов (30 минут)
- 📝 Расширено логирование действий администраторов
- 🐛 Исправлены проблемы с CORS для frontend портов

### Version 1.0.0 (2024-10-29)
- 🚀 Первый стабильный релиз
- 🏗️ 9 микросервисов
- 🐳 Docker Compose конфигурация
- 📚 Полная документация по деплою
