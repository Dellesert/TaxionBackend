# Development vs Production Configuration

Сравнение конфигураций между development и production окружениями.

## Файлы конфигурации

| Environment | Файл | Использование |
|-------------|------|---------------|
| Development | `.env` | `docker-compose.yml` |
| Production | `.env.production.local` | `docker-compose.prod.yml` |

## Ключевые различия

### 1. Environment Settings

| Параметр | Development | Production |
|----------|-------------|------------|
| `ENVIRONMENT` | `development` | `production` |
| `GIN_MODE` | `debug` | `release` |
| `DEBUG` | `true` | `false` |
| `LOG_LEVEL` | `debug/info` | `info/warn` |

### 2. Security

| Параметр | Development | Production |
|----------|-------------|------------|
| `SESSION_DURATION_HOURS` | 168 (7 дней) | 0.5 (30 минут) |
| `POSTGRES_PASSWORD` | Простой пароль | Сильный пароль 32+ символов |
| `REDIS_PASSWORD` | Простой пароль | Сильный пароль 32+ символов |
| `SUPER_ADMIN_PASSWORD` | Простой | Сильный, меняется после входа |
| SSL Mode (PostgreSQL) | `sslmode=disable` | `sslmode=require` |

### 3. CORS

| Параметр | Development | Production |
|----------|-------------|------------|
| `CORS_ORIGINS` | Множество локальных URL | Только production домены |
| Origins | `localhost:*`, IP адреса | `https://yourdomain.com` |

### 4. WebAuthn

| Параметр | Development | Production |
|----------|-------------|------------|
| `WEBAUTHN_RP_ID` | `localhost` | `yourdomain.com` |
| `WEBAUTHN_RP_ORIGIN` | `http://localhost:*` | `https://yourdomain.com` |
| Protocol | HTTP разрешен | Только HTTPS |

### 5. Database

| Параметр | Development | Production |
|----------|-------------|------------|
| `ENABLE_SQL_LOGGING` | `true` | `false` |
| `DB_MAX_OPEN_CONNS` | 25 | 50 |
| `DB_MAX_IDLE_CONNS` | 5 | 10 |
| External Access | Порт открыт (5432) | Порт закрыт |

### 6. Redis

| Параметр | Development | Production |
|----------|-------------|------------|
| `REDIS_MAX_ACTIVE` | 100 | 200 |
| `REDIS_MAX_IDLE` | 10 | 20 |
| External Access | Порт открыт (6379) | Порт закрыт |

### 7. Docker Resources

| Сервис | Development | Production |
|--------|-------------|------------|
| CPU Limits | Не ограничено | Ограничено (см. docker-compose.prod.yml) |
| Memory Limits | Не ограничено | Ограничено |
| Restart Policy | `unless-stopped` | `always` |

### 8. Logging

| Параметр | Development | Production |
|----------|-------------|------------|
| `ENABLE_REQUEST_LOGGING` | `true` | `true` |
| `ENABLE_SQL_LOGGING` | `true` | `false` (производительность) |
| Log Rotation | Нет | Да (max-size: 10m, max-file: 5) |
| Log Format | `json` | `json` |

### 9. Networking

| Параметр | Development | Production |
|----------|-------------|------------|
| External Ports | Все сервисы | Только Gateway (8080) |
| Network Name | `tachyon-network` | `tachyon-network-prod` |
| Container Names | `tachyon-*` | `tachyon-*-prod` |

### 10. Monitoring & Health

| Параметр | Development | Production |
|----------|-------------|------------|
| Health Check Interval | 30s | 30s |
| Health Check Start Period | 40s | 60s |
| Metrics | Опционально | Рекомендуется |

## Миграция с Development на Production

### Шаг 1: Подготовка конфигурации

```bash
# Копируем production template
cp .env.production .env.production.local

# Генерируем пароли
openssl rand -base64 32  # POSTGRES_PASSWORD
openssl rand -base64 32  # REDIS_PASSWORD
```

### Шаг 2: Изменение критических параметров

1. **Пароли**: Замените все пароли на сильные
2. **Домены**: Укажите реальные production домены
3. **CORS**: Ограничьте только production URL
4. **Session Duration**: Уменьшите до 30 минут
5. **Admin credentials**: Измените и смените после входа

### Шаг 3: Экспорт данных

```bash
# Backup development данных (если нужно)
docker exec tachyon-postgres pg_dump -U tachyon_user tachyon_messenger > dev_backup.sql

# Импорт в production (опционально)
cat dev_backup.sql | docker exec -i tachyon-postgres-prod psql -U tachyon_user tachyon_messenger
```

### Шаг 4: Запуск production

```bash
# Запуск с production конфигурацией
docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d --build
```

## Рекомендации

### Development

- ✅ Используйте простые пароли для удобства
- ✅ Открывайте порты для отладки
- ✅ Включайте подробное логирование
- ✅ Разрешайте CORS для всех локальных URL
- ⚠️ Не используйте `.env` в production!

### Production

- ⚠️ **Обязательно** используйте сильные пароли
- ⚠️ **Закрывайте** прямой доступ к БД и Redis
- ⚠️ **Ограничьте** CORS только production доменами
- ⚠️ **Используйте** HTTPS везде
- ⚠️ **Настройте** автоматические backup
- ⚠️ **Мониторьте** логи и ресурсы
- ⚠️ **Обновляйте** систему регулярно

## Чеклист перед запуском Production

- [ ] Изменены все пароли на сильные (32+ символов)
- [ ] Настроен правильный домен в `WEBAUTHN_RP_ID`
- [ ] CORS ограничен только production URL
- [ ] SSL/TLS настроен (HTTPS)
- [ ] Закрыты внешние порты БД и Redis
- [ ] Настроен Nginx reverse proxy
- [ ] Получен SSL сертификат (Let's Encrypt)
- [ ] Настроены автоматические backup
- [ ] Проверены health checks всех сервисов
- [ ] Настроен мониторинг
- [ ] Firewall настроен (только 80, 443, 22)
- [ ] Изменен пароль super admin после первого входа
- [ ] Протестирована функциональность
- [ ] Настроено логирование и ротация
- [ ] Документированы production credentials (в безопасном месте!)

## Troubleshooting

### "Invalid origin" ошибка

**Причина**: CORS или WebAuthn origin не совпадает

**Решение**:
```env
CORS_ORIGINS=https://yourdomain.com
WEBAUTHN_RP_ORIGIN=https://yourdomain.com
```

### "Cannot connect to database"

**Причина**: Неправильный пароль или SSL mode

**Решение**:
```env
# Production должен использовать SSL
DATABASE_URL=postgres://user:pass@postgres:5432/db?sslmode=require
```

### WebSocket не работает через Nginx

**Причина**: Не настроены WebSocket headers

**Решение**: См. конфигурацию Nginx в `PRODUCTION_DEPLOYMENT.md`

### Сессии не сохраняются

**Причина**: Redis недоступен или неправильный пароль

**Решение**:
```bash
# Проверьте Redis
docker exec -it tachyon-redis-prod redis-cli -a "YOUR_PASSWORD" ping
```

## Дополнительные ресурсы

- [Production Quick Start](PRODUCTION_QUICKSTART.md)
- [Full Production Guide](PRODUCTION_DEPLOYMENT.md)
- [Main README](README.md)
