# Environment Configuration Guide

Руководство по настройке переменных окружения для Tachyon Messenger.

## Файлы конфигурации

| Файл | Назначение | Коммитить в Git? |
|------|------------|------------------|
| `.env.example` | Шаблон для development | ✅ Да |
| `.env` | Development конфигурация | ❌ Нет |
| `.env.production.example` | Шаблон для production | ✅ Да |
| `.env.production.local` | Production конфигурация | ❌ НИКОГДА! |

## Quick Start

### Development Setup

```bash
# 1. Копируем development template
cp .env.example .env

# 2. (Опционально) Изменяем настройки под себя
nano .env

# 3. Запускаем
docker-compose up -d
```

### Production Setup

```bash
# 1. Копируем production template
cp .env.production.example .env.production.local

# 2. ОБЯЗАТЕЛЬНО изменяем критичные параметры
nano .env.production.local

# 3. Запускаем production
docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d --build
```

## Критически важные параметры

### 🔐 Security (изменить ОБЯЗАТЕЛЬНО!)

```env
# Сгенерируйте сильные пароли:
openssl rand -base64 32

POSTGRES_PASSWORD=CHANGE_ME_32_CHARS_MIN
REDIS_PASSWORD=CHANGE_ME_32_CHARS_MIN
SUPER_ADMIN_PASSWORD=CHANGE_ME_STRONG_PASSWORD
```

### 🌐 Domain Configuration

```env
# Ваш production домен (БЕЗ https://)
WEBAUTHN_RP_ID=yourdomain.com

# Origin с протоколом (ТОЛЬКО https в production!)
WEBAUTHN_RP_ORIGIN=https://yourdomain.com

# CORS (разделяйте запятой БЕЗ пробелов)
CORS_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

# Base URL для API и файлов
BASE_URL=https://yourdomain.com
FRONTEND_URL=https://yourdomain.com
```

### 📧 Email Configuration

```env
# Для Gmail используйте App Password
# https://myaccount.google.com/apppasswords

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_FROM_EMAIL=noreply@yourdomain.com
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-gmail-app-password
```

## Параметры по категориям

### Environment Mode

| Параметр | Development | Production | Описание |
|----------|-------------|------------|----------|
| `ENVIRONMENT` | `development` | `production` | Режим окружения |
| `GIN_MODE` | `debug` | `release` | Gin framework mode |
| `DEBUG` | `true` | `false` | Debug режим |
| `LOG_LEVEL` | `debug` | `info` | Уровень логирования |

### Authentication

| Параметр | Значение | Описание |
|----------|----------|----------|
| `AUTH_MODE` | `session` или `jwt` | Режим аутентификации |
| `SESSION_DURATION_HOURS` | `168` (dev) / `0.5` (prod) | Длительность сессии |

**Рекомендации:**
- Production: `AUTH_MODE=session` + `SESSION_DURATION_HOURS=0.5` (30 минут)
- Development: `AUTH_MODE=session` + `SESSION_DURATION_HOURS=168` (7 дней)

### Database

```env
# Development
DATABASE_URL=postgres://user:pass@postgres:5432/db?sslmode=disable

# Production (ОБЯЗАТЕЛЬНО с SSL!)
DATABASE_URL=postgres://user:pass@postgres:5432/db?sslmode=require
```

**Performance Settings:**

| Параметр | Development | Production |
|----------|-------------|------------|
| `DB_MAX_OPEN_CONNS` | 25 | 50 |
| `DB_MAX_IDLE_CONNS` | 5 | 10 |
| `ENABLE_SQL_LOGGING` | `true` | `false` |

### Redis

```env
REDIS_URL=redis://:PASSWORD@redis:6379
```

**Performance Settings:**

| Параметр | Development | Production |
|----------|-------------|------------|
| `REDIS_MAX_ACTIVE` | 100 | 200 |
| `REDIS_MAX_IDLE` | 10 | 20 |

### CORS

```env
# Development - разрешаем все локальные URL
CORS_ORIGINS=http://localhost:3000,http://localhost:5173,http://192.168.0.189:8080

# Production - ТОЛЬКО production домены!
CORS_ORIGINS=https://yourdomain.com
```

### File Upload

```env
UPLOAD_DIR=/app/uploads
MAX_UPLOAD_SIZE=10485760  # 10MB в байтах
BASE_URL=https://yourdomain.com
```

### Notifications

```env
NOTIFICATION_CONCURRENT_WORKERS=5   # Development
NOTIFICATION_CONCURRENT_WORKERS=10  # Production

NOTIFICATION_QUEUE_SIZE=1000        # Development
NOTIFICATION_QUEUE_SIZE=2000        # Production
```

## Генерация безопасных значений

### Пароли

```bash
# PostgreSQL password
openssl rand -base64 32

# Redis password
openssl rand -base64 32

# Super Admin password (минимум 16 символов)
openssl rand -base64 24
```

### JWT Secret (если используете JWT mode)

```bash
openssl rand -base64 64
```

## Проверка конфигурации

### 1. Проверка переменных окружения

```bash
# Development
docker-compose config

# Production
docker compose -f docker-compose.prod.yml --env-file .env.production.local config
```

### 2. Проверка запуска

```bash
# Проверяем логи при старте
docker logs tachyon-user-service 2>&1 | grep -E "AUTH_MODE|DATABASE_URL|REDIS_URL"

# Должно показать:
# ℹ️  JWT_SECRET not set (not required for session mode)
# DATABASE_URL: postgres:/***
# AUTH_MODE: session
# SESSION_DURATION: 168 hours
```

### 3. Health Check

```bash
# Проверяем здоровье сервисов
curl http://localhost:8080/health
curl http://localhost:8080/health/services
```

## Troubleshooting

### "JWT_SECRET is required"

**Проблема:** Система требует JWT_SECRET даже в session mode

**Решение:** Убедитесь что:
```env
AUTH_MODE=session
```

### "Invalid origin" при WebAuthn

**Проблема:** Origin не совпадает

**Решение:**
```env
# Development
WEBAUTHN_RP_ID=localhost
WEBAUTHN_RP_ORIGIN=http://localhost:5173

# Production
WEBAUTHN_RP_ID=yourdomain.com
WEBAUTHN_RP_ORIGIN=https://yourdomain.com
```

### CORS ошибки

**Проблема:** Frontend не может подключиться

**Решение:**
```env
# Проверьте что origin есть в списке (БЕЗ пробелов!)
CORS_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
```

### Email не отправляются

**Проблема:** SMTP ошибки

**Решение для Gmail:**
1. Включите 2FA: https://myaccount.google.com/security
2. Создайте App Password: https://myaccount.google.com/apppasswords
3. Используйте App Password в `SMTP_PASSWORD`

## Best Practices

### ✅ DO

- ✅ Используйте сильные пароли (32+ символов)
- ✅ Генерируйте пароли через `openssl rand -base64 32`
- ✅ В production используйте `sslmode=require` для PostgreSQL
- ✅ Ограничивайте CORS только необходимыми доменами
- ✅ Используйте HTTPS в production (обязательно!)
- ✅ Регулярно меняйте пароли
- ✅ Храните `.env.production.local` в безопасном месте
- ✅ Делайте backup конфигурации

### ❌ DON'T

- ❌ НЕ коммитьте `.env` или `.env.production.local` в Git
- ❌ НЕ используйте слабые пароли
- ❌ НЕ используйте HTTP в production
- ❌ НЕ открывайте CORS для всех (`*`)
- ❌ НЕ храните секреты в открытом виде
- ❌ НЕ используйте одинаковые пароли для dev и prod
- ❌ НЕ используйте `sslmode=disable` в production

## Security Checklist

- [ ] Изменены все пароли на сильные (32+ символов)
- [ ] `AUTH_MODE=session` для production
- [ ] `SESSION_DURATION_HOURS=0.5` для production (30 минут)
- [ ] CORS ограничен только production доменами
- [ ] PostgreSQL использует `sslmode=require`
- [ ] HTTPS настроен (Let's Encrypt)
- [ ] Email SMTP настроен с App Password
- [ ] Super Admin пароль изменен после первого входа
- [ ] `.env.production.local` НЕ в Git
- [ ] Backup конфигурации в безопасном месте

## Дополнительные ресурсы

- [Production Quick Start](PRODUCTION_QUICKSTART.md)
- [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
- [Environment Comparison](ENVIRONMENT_COMPARISON.md)
- [Main README](README.md)
