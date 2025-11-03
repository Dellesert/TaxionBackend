# Production Deployment Guide

Инструкция по развертыванию Tachyon Messenger в production окружении.

## Предварительная подготовка

### 1. Настройка сервера

Минимальные требования:
- **CPU**: 4+ ядер
- **RAM**: 8+ GB
- **Disk**: 50+ GB SSD
- **OS**: Ubuntu 22.04 LTS или аналог
- **Docker**: 24.0+
- **Docker Compose**: 2.20+

### 2. Установка зависимостей

```bash
# Обновление системы
sudo apt update && sudo apt upgrade -y

# Установка Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Установка Docker Compose
sudo apt install docker-compose-plugin -y

# Проверка установки
docker --version
docker compose version
```

### 3. Настройка firewall

```bash
# Разрешаем только необходимые порты
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS
sudo ufw allow 8080/tcp  # Gateway (временно, затем настроить nginx)
sudo ufw enable
```

## Конфигурация

### 1. Клонирование репозитория

```bash
cd /opt
sudo git clone https://github.com/your-org/TaxionBackend.git
cd TaxionBackend
```

### 2. Настройка .env.production

```bash
# Копируем production template
cp .env.production.example .env.production.local

# КРИТИЧЕСКИ ВАЖНО: Измените все пароли и секреты!
nano .env.production.local
```

**Обязательно измените:**

1. **Пароли базы данных:**
```env
POSTGRES_PASSWORD=ВАШ_СИЛЬНЫЙ_ПАРОЛЬ_MIN_32_СИМВОЛА
REDIS_PASSWORD=ВАШ_СИЛЬНЫЙ_REDIS_ПАРОЛЬ_MIN_32_СИМВОЛА
```

2. **Super Admin credentials:**
```env
SUPER_ADMIN_EMAIL=admin@yourdomain.com
SUPER_ADMIN_PASSWORD=СИЛЬНЫЙ_ПАРОЛЬ_ИЗМЕНИТЕ_ПОСЛЕ_ВХОДА
```

3. **Domain settings:**
```env
WEBAUTHN_RP_ID=yourdomain.com
WEBAUTHN_RP_ORIGIN=https://yourdomain.com
BASE_URL=https://yourdomain.com
FRONTEND_URL=https://yourdomain.com
```

4. **CORS origins:**
```env
CORS_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
```

### 3. Генерация сильных паролей

```bash
# Генерация случайных паролей (используйте для POSTGRES_PASSWORD и REDIS_PASSWORD)
openssl rand -base64 32
openssl rand -base64 32
```

## Развертывание

### 1. Сборка и запуск

```bash
# Используем .env.production.local
docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d --build
```

### 2. Проверка статуса

```bash
# Проверяем статус всех контейнеров
docker ps

# Проверяем логи gateway
docker logs tachyon-gateway-prod -f

# Проверяем health всех сервисов
docker compose -f docker-compose.prod.yml ps
```

### 3. Проверка работоспособности

```bash
# Health check API Gateway
curl http://localhost:8080/health

# Health check всех сервисов
curl http://localhost:8080/health/services
```

## Настройка Nginx (Reverse Proxy)

### 1. Установка Nginx

```bash
sudo apt install nginx -y
```

### 2. Конфигурация Nginx

Создайте файл `/etc/nginx/sites-available/tachyon`:

```nginx
# HTTP -> HTTPS redirect
server {
    listen 80;
    listen [::]:80;
    server_name yourdomain.com;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name yourdomain.com;

    # SSL certificates (Let's Encrypt)
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Max upload size
    client_max_body_size 10M;

    # WebSocket support
    location /api/v1/ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;

        # WebSocket headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket timeouts
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;

        # Disable buffering
        proxy_buffering off;
    }

    # API endpoints
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:8080;
        access_log off;
    }

    # Frontend (если используете)
    location / {
        root /var/www/tachyon/frontend;
        try_files $uri $uri/ /index.html;
    }
}
```

### 3. Активация конфигурации

```bash
# Создаем симлинк
sudo ln -s /etc/nginx/sites-available/tachyon /etc/nginx/sites-enabled/

# Проверяем конфигурацию
sudo nginx -t

# Перезапускаем Nginx
sudo systemctl restart nginx
```

## SSL сертификат (Let's Encrypt)

### 1. Установка Certbot

```bash
sudo apt install certbot python3-certbot-nginx -y
```

### 2. Получение сертификата

```bash
sudo certbot --nginx -d yourdomain.com
```

### 3. Автоматическое обновление

```bash
# Проверяем автообновление
sudo certbot renew --dry-run
```

## Мониторинг

### 1. Просмотр логов

```bash
# Все сервисы
docker compose -f docker-compose.prod.yml logs -f

# Конкретный сервис
docker logs tachyon-gateway-prod -f

# Последние 100 строк
docker logs tachyon-user-service-prod --tail 100
```

### 2. Мониторинг ресурсов

```bash
# Статистика контейнеров
docker stats

# Использование дисков
df -h
docker system df
```

## Backup

### 1. Backup базы данных

```bash
# Создаем директорию для бэкапов
mkdir -p /opt/TaxionBackend/backups/postgres

# Backup PostgreSQL
docker exec tachyon-postgres-prod pg_dump -U tachyon_user tachyon_messenger > /opt/TaxionBackend/backups/postgres/backup_$(date +%Y%m%d_%H%M%S).sql

# Backup Redis (RDB)
docker exec tachyon-redis-prod redis-cli --no-auth-warning -a "$REDIS_PASSWORD" BGSAVE
docker cp tachyon-redis-prod:/data/dump.rdb /opt/TaxionBackend/backups/redis/dump_$(date +%Y%m%d_%H%M%S).rdb
```

### 2. Автоматический backup (cron)

```bash
# Создаем скрипт backup
sudo nano /opt/TaxionBackend/backup.sh
```

```bash
#!/bin/bash
BACKUP_DIR="/opt/TaxionBackend/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# PostgreSQL backup
docker exec tachyon-postgres-prod pg_dump -U tachyon_user tachyon_messenger | gzip > $BACKUP_DIR/postgres/backup_$DATE.sql.gz

# Redis backup
docker exec tachyon-redis-prod redis-cli --no-auth-warning -a "$REDIS_PASSWORD" BGSAVE
docker cp tachyon-redis-prod:/data/dump.rdb $BACKUP_DIR/redis/dump_$DATE.rdb

# Удаляем старые бэкапы (старше 30 дней)
find $BACKUP_DIR/postgres -name "backup_*.sql.gz" -mtime +30 -delete
find $BACKUP_DIR/redis -name "dump_*.rdb" -mtime +30 -delete

echo "Backup completed: $DATE"
```

```bash
# Делаем скрипт исполняемым
sudo chmod +x /opt/TaxionBackend/backup.sh

# Добавляем в crontab (каждый день в 3:00)
sudo crontab -e
# Добавьте строку:
0 3 * * * /opt/TaxionBackend/backup.sh >> /opt/TaxionBackend/backups/backup.log 2>&1
```

## Обновление

### 1. Обновление кода

```bash
cd /opt/TaxionBackend

# Останавливаем сервисы
docker compose -f docker-compose.prod.yml down

# Обновляем код
git pull origin main

# Пересобираем и запускаем
docker compose -f docker-compose.prod.yml up -d --build
```

### 2. Безопасное обновление (zero downtime)

```bash
# Пересобираем образы
docker compose -f docker-compose.prod.yml build

# Обновляем по одному сервису
docker compose -f docker-compose.prod.yml up -d --no-deps --build user-service
docker compose -f docker-compose.prod.yml up -d --no-deps --build chat-service
# ... и так далее
```

## Troubleshooting

### Проблема: Сервис не стартует

```bash
# Проверяем логи
docker logs tachyon-SERVICE-NAME-prod

# Проверяем статус
docker compose -f docker-compose.prod.yml ps

# Перезапускаем конкретный сервис
docker compose -f docker-compose.prod.yml restart SERVICE-NAME
```

### Проблема: База данных недоступна

```bash
# Проверяем соединение с PostgreSQL
docker exec -it tachyon-postgres-prod psql -U tachyon_user -d tachyon_messenger

# Проверяем соединение с Redis
docker exec -it tachyon-redis-prod redis-cli -a "$REDIS_PASSWORD" ping
```

### Проблема: Нехватка памяти

```bash
# Проверяем использование памяти
docker stats

# Очищаем неиспользуемые образы и контейнеры
docker system prune -a
```

## Security Checklist

- [ ] Изменены все пароли по умолчанию
- [ ] Настроен firewall (ufw)
- [ ] Установлен SSL сертификат
- [ ] Включен HSTS
- [ ] Настроен автоматический backup
- [ ] Закрыты внешние порты (кроме 80, 443)
- [ ] Изменен пароль super admin после первого входа
- [ ] Настроен мониторинг
- [ ] Регулярные обновления системы
- [ ] Логи ротируются
- [ ] Ограничены ресурсы контейнеров

## Support

При возникновении проблем:
1. Проверьте логи: `docker compose -f docker-compose.prod.yml logs`
2. Проверьте health: `curl http://localhost:8080/health/services`
3. Обратитесь к документации: `/opt/TaxionBackend/README.md`
