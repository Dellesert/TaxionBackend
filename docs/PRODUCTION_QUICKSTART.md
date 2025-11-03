# Production Quick Start

Быстрый старт для развертывания в production.

## 1. Подготовка (5 минут)

```bash
# Клонируем репозиторий
cd /opt
git clone <your-repo-url> TaxionBackend
cd TaxionBackend

# Создаем директории
mkdir -p logs/{gateway,user,chat,task,calendar,poll,notification,file}
mkdir -p backups/{postgres,redis}
```

## 2. Конфигурация (10 минут)

```bash
# Копируем production template
cp .env.production.example .env.production.local

# Редактируем конфигурацию
nano .env.production.local
```

### Обязательно измените:

1. **Пароли:**
```bash
# Генерируем сильные пароли
openssl rand -base64 32  # Для POSTGRES_PASSWORD
openssl rand -base64 32  # Для REDIS_PASSWORD
```

2. **Домены:**
```env
WEBAUTHN_RP_ID=yourdomain.com
WEBAUTHN_RP_ORIGIN=https://yourdomain.com
BASE_URL=https://yourdomain.com
CORS_ORIGINS=https://yourdomain.com
```

3. **Admin:**
```env
SUPER_ADMIN_EMAIL=admin@yourdomain.com
SUPER_ADMIN_PASSWORD=StrongPassword123!@#
```

## 3. Запуск (5 минут)

```bash
# Запускаем production
docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d --build

# Проверяем статус
docker compose -f docker-compose.prod.yml ps

# Проверяем логи
docker compose -f docker-compose.prod.yml logs -f gateway
```

## 4. Проверка (2 минуты)

```bash
# Health check
curl http://localhost:8080/health

# Проверка всех сервисов
curl http://localhost:8080/health/services
```

## 5. Настройка Nginx + SSL (10 минут)

### Установка

```bash
sudo apt update
sudo apt install nginx certbot python3-certbot-nginx -y
```

### Конфигурация

```bash
# Копируем конфигурацию из PRODUCTION_DEPLOYMENT.md
sudo nano /etc/nginx/sites-available/tachyon

# Активируем
sudo ln -s /etc/nginx/sites-available/tachyon /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

### SSL сертификат

```bash
sudo certbot --nginx -d yourdomain.com
```

## 6. Первый вход

1. Откройте: `https://yourdomain.com`
2. Войдите как Super Admin
3. **СРАЗУ измените пароль!**

## 7. Автоматический Backup (5 минут)

```bash
# Создаем скрипт
sudo nano /opt/TaxionBackend/backup.sh
```

Скопируйте скрипт из `PRODUCTION_DEPLOYMENT.md` → секция Backup

```bash
# Делаем исполняемым
sudo chmod +x /opt/TaxionBackend/backup.sh

# Добавляем в cron (каждый день в 3:00)
sudo crontab -e
# Добавьте:
0 3 * * * /opt/TaxionBackend/backup.sh >> /opt/TaxionBackend/backups/backup.log 2>&1
```

## Готово! 🎉

Ваш Tachyon Messenger работает в production!

## Полезные команды

```bash
# Просмотр логов
docker compose -f docker-compose.prod.yml logs -f

# Перезапуск сервиса
docker compose -f docker-compose.prod.yml restart gateway

# Обновление
git pull origin main
docker compose -f docker-compose.prod.yml up -d --build

# Backup вручную
./backup.sh

# Статистика ресурсов
docker stats
```

## Важные замечания

- ⚠️ **Измените пароль super admin** после первого входа!
- ⚠️ **Настройте регулярные backup**
- ⚠️ **Мониторьте логи**: `docker compose -f docker-compose.prod.yml logs`
- ⚠️ **Обновляйте систему**: `sudo apt update && sudo apt upgrade`

## Troubleshooting

**Сервис не стартует:**
```bash
docker logs tachyon-SERVICENAME-prod
docker compose -f docker-compose.prod.yml restart SERVICENAME
```

**502 Bad Gateway:**
```bash
# Проверьте статус gateway
docker logs tachyon-gateway-prod
# Проверьте nginx
sudo nginx -t
sudo systemctl status nginx
```

**База данных недоступна:**
```bash
docker exec -it tachyon-postgres-prod psql -U tachyon_user -d tachyon_messenger
```

Полная документация: [PRODUCTION_DEPLOYMENT.md](PRODUCTION_DEPLOYMENT.md)
