# ПОЛНЫЙ ЧЕКЛИСТ ДЕПЛОЯ TACHYON MESSENGER

**Версия**: 1.0
**Дата**: 2025-11-20
**Проект**: Tachyon Messenger Backend

---

## I. ТРЕБОВАНИЯ К СЕРВЕРУ

### 1. Системные требования

- **CPU**: 4+ ядра (минимум 2)
- **RAM**: 8+ GB (минимум 4 GB)
- **Диск**: 50+ GB SSD (минимум 10 GB)
- **ОС**: Ubuntu 22.04 LTS (рекомендуется) или Ubuntu 20.04+
- **Интернет**: стабильное подключение

### 2. Установленное ПО

- [ ] Docker 24.0+ (`docker --version`)
- [ ] Docker Compose 2.20+ (`docker compose version`)
- [ ] Git (`git --version`)
- [ ] Nginx (для reverse proxy и SSL)
- [ ] Certbot + python3-certbot-nginx (для SSL сертификатов)
- [ ] UFW или iptables (firewall)

### 3. Сетевые требования

- [ ] Открыты порты в firewall:
  - 22 (SSH)
  - 80 (HTTP)
  - 443 (HTTPS)
  - 8080 (временно, для тестирования backend напрямую)
- [ ] Настроен firewall (UFW):

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

---

## II. ДОМЕНЫ И SSL

### 1. Домены

- [ ] **Основной домен** для backend API (например: `api.taxion.ru` или `taxion.ru`)
  - DNS A-запись настроена и указывает на IP сервера
  - Проверить: `ping api.taxion.ru`

- [ ] **Дополнительный домен** (опционально):
  - Для админ-панели: `admin.taxion.ru`
  - **ИЛИ** использовать локальный доступ через SSH туннель (безопаснее):

```bash
ssh -L 8080:localhost:8080 user@server
# Затем открыть http://localhost:8080 локально
```

### 2. SSL сертификаты

- [ ] SSL сертификат от Let's Encrypt установлен для всех доменов

```bash
sudo certbot --nginx -d api.taxion.ru -d admin.taxion.ru
```

- [ ] Автообновление сертификатов настроено (проверить: `sudo certbot renew --dry-run`)
- [ ] Nginx настроен на redirect HTTP → HTTPS
- [ ] HSTS заголовки включены в Nginx

---

## III. SMTP И EMAIL

### 1. Данные SMTP сервера

- [ ] **SMTP Host**: `_________________` (например: `smtp.gmail.com` или корпоративный)
- [ ] **SMTP Port**: `587` (TLS) или `465` (SSL)
- [ ] **SMTP User**: `_________________`
- [ ] **SMTP Password**: `_________________` (для Gmail - App Password!)
- [ ] **FROM Email**: `noreply@taxion.ru` или корпоративный email
- [ ] **FROM Name**: `Taxion Messenger`

### 2. Тестирование SMTP

- [ ] Проверить подключение до деплоя:

```bash
telnet smtp.gmail.com 587
# или
openssl s_client -connect smtp.gmail.com:465 -crlf
```

- [ ] Получить App Password для Gmail (если используется Gmail):
  - https://myaccount.google.com/apppasswords

### 3. Альтернативы SMTP

Если корпоративного SMTP нет, можно использовать:

- **Gmail** (бесплатно, до 500 писем/день)
- **SendGrid** (12,000 писем/месяц бесплатно)
- **Mailgun** (5,000 писем/месяц бесплатно)
- **AWS SES** (62,000 писем/месяц бесплатно с EC2)

---

## IV. ДАННЫЕ ДЛЯ ЗАПОЛНЕНИЯ

### 1. Пользователи и отделы

- [ ] **CSV файл с пользователями** готов (если есть)
  - Формат: `email,name,phone,department,role`
  - Пример пути: `scripts/seed/data/users.csv`

- [ ] **CSV файл с отделами** готов (если есть)
  - Формат: `department_name,parent_department`
  - Пример пути: `scripts/seed/data/departments.csv`

- [ ] **ИЛИ** использовать встроенный генератор моковых данных:

```bash
go run scripts/seed/main.go --all --clean \
  --user-count 100 \
  --chat-count 50 \
  --task-count 200
```

### 2. Начальные данные администратора

- [ ] Email суперадмина: `_________________`
- [ ] Временный пароль: `_________________` (минимум 16 символов, изменить после входа!)
- [ ] Имя суперадмина: `_________________`

---

## V. КОНФИГУРАЦИОННЫЕ ДАННЫЕ

### 1. Генерация секретных паролей

```bash
# Генерируем 3 сильных пароля
openssl rand -base64 32  # Для POSTGRES_PASSWORD
openssl rand -base64 32  # Для REDIS_PASSWORD
openssl rand -base64 32  # Для SUPER_ADMIN_PASSWORD (начальный)
```

### 2. Production .env файл (.env.production.local)

Обязательные переменные для настройки:

- [ ] `ENVIRONMENT=production`
- [ ] `GIN_MODE=release`
- [ ] `POSTGRES_PASSWORD` - сгенерированный пароль
- [ ] `REDIS_PASSWORD` - сгенерированный пароль
- [ ] `DATABASE_URL` - с sslmode=require
- [ ] `WEBAUTHN_RP_ID` - домен без https:// (например: `taxion.ru`)
- [ ] `WEBAUTHN_RP_ORIGIN` - полный URL с https:// (например: `https://taxion.ru`)
- [ ] `BACKEND_URL` - `https://taxion.ru`
- [ ] `FRONTEND_URL` - `https://taxion.ru`
- [ ] `CORS_ORIGINS` - список разрешенных доменов через запятую
- [ ] `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_FROM_NAME`
- [ ] `SUPER_ADMIN_EMAIL`, `SUPER_ADMIN_PASSWORD`, `SUPER_ADMIN_NAME`
- [ ] `FCM_ENABLED=false` (пока не настроено)

---

## VI. ПОДГОТОВКА К ДЕПЛОЮ (СДЕЛАТЬ ЗАРАНЕЕ)

### 1. На локальной машине

- [ ] Обновить код в git (если есть изменения)
- [ ] Протестировать docker-compose.prod.yml локально
- [ ] Подготовить .env.production.local с реальными данными
- [ ] Подготовить Nginx конфигурацию (файл готов в README.md)
- [ ] Подготовить backup скрипт (файл готов в docs/)

### 2. Подготовить файлы для копирования

```bash
# Создать папку с файлами для деплоя
mkdir deploy-taxion
cd deploy-taxion

# Положить файлы:
# - .env.production.local (заполненный!)
# - nginx-config.txt (конфигурация Nginx из README.md)
# - backup.sh (скрипт бэкапа)
```

---

## VII. ПРОЦЕСС ДЕПЛОЯ НА СЕРВЕРЕ

### Шаг 1: Подготовка сервера (15 минут)

```bash
# 1. Обновление системы
sudo apt update && sudo apt upgrade -y

# 2. Установка Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
newgrp docker

# 3. Установка Docker Compose
sudo apt install docker-compose-plugin -y

# 4. Установка Nginx и Certbot
sudo apt install nginx certbot python3-certbot-nginx -y

# 5. Настройка firewall
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable

# 6. Проверка установки
docker --version
docker compose version
nginx -v
```

### Шаг 2: Клонирование проекта (5 минут)

```bash
# Клонируем в /opt (рекомендуется для production)
cd /opt
sudo git clone https://github.com/mishkajackson/Taxion.git TaxionBack
cd TaxionBack

# Устанавливаем права
sudo chown -R $USER:$USER /opt/TaxionBack

# Создаем директории
mkdir -p logs/{gateway,user,chat,task,calendar,poll,notification,file}
mkdir -p backups/{postgres,redis}
mkdir -p credentials
```

### Шаг 3: Настройка конфигурации (10 минут)

```bash
# 1. Копируем заранее подготовленный .env.production.local
# (с локальной машины через scp или создаем на месте)
nano .env.production.local
# Вставить заранее подготовленное содержимое

# 2. Проверяем, что все пароли изменены
grep "CHANGE_ME" .env.production.local  # Не должно найти ничего!

# 3. Проверяем наличие init.sql
ls -la init.sql
```

### Шаг 4: Запуск сервисов (10 минут)

```bash
# 1. Запускаем production
docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d --build

# 2. Ждем инициализацию (30-60 секунд)
sleep 60

# 3. Проверяем статус
docker compose -f docker-compose.prod.yml ps

# 4. Проверяем логи
docker compose -f docker-compose.prod.yml logs -f gateway
# Ждем сообщение "Server started on :8080"
# Ctrl+C чтобы выйти

# 5. Проверяем health
curl http://localhost:8080/health
curl http://localhost:8080/health/services
```

### Шаг 5: Настройка Nginx (10 минут)

```bash
# 1. Создаем конфигурацию
sudo nano /etc/nginx/sites-available/taxion
# Вставить конфигурацию из README.md (секция "Настройка Nginx")
# ЗАМЕНИТЬ yourdomain.com на ваш реальный домен!

# 2. Активируем сайт
sudo ln -s /etc/nginx/sites-available/taxion /etc/nginx/sites-enabled/
sudo rm /etc/nginx/sites-enabled/default  # Удаляем дефолтный сайт

# 3. Проверяем конфигурацию
sudo nginx -t

# 4. Перезапускаем Nginx
sudo systemctl restart nginx
```

### Шаг 6: Получение SSL сертификата (5 минут)

```bash
# 1. Получаем сертификат от Let's Encrypt
sudo certbot --nginx -d api.taxion.ru

# Следуем инструкциям:
# - Ввести email
# - Согласиться с ToS
# - Выбрать "2: Redirect" для автоматического HTTPS redirect

# 2. Проверяем автообновление
sudo certbot renew --dry-run

# 3. Тестируем HTTPS
curl https://api.taxion.ru/health
```

### Шаг 7: Заполнение данных (10 минут)

```bash
# Вариант 1: Использовать готовые CSV (если есть)
# TODO: добавить скрипт импорта из CSV

# Вариант 2: Сгенерировать моковые данные
cd /opt/TaxionBack
go run scripts/seed/main.go --all --clean \
  --user-count 100 \
  --chat-count 50 \
  --task-count 200 \
  --poll-count 30 \
  --event-count 100
```

### Шаг 8: Настройка автоматического backup (5 минут)

```bash
# 1. Создаем скрипт
sudo nano /opt/TaxionBack/backup.sh
# Вставить содержимое из PRODUCTION_DEPLOYMENT.md

# 2. Делаем исполняемым
sudo chmod +x /opt/TaxionBack/backup.sh

# 3. Тестируем
./backup.sh

# 4. Добавляем в cron (каждый день в 3:00 ночи)
sudo crontab -e
# Добавить строку:
# 0 3 * * * /opt/TaxionBack/backup.sh >> /opt/TaxionBack/backups/backup.log 2>&1
```

### Шаг 9: Настройка автозапуска (5 минут)

```bash
# Создаем systemd service
sudo nano /etc/systemd/system/taxion.service
```

**Содержимое файла:**

```ini
[Unit]
Description=Taxion Messenger Backend
Requires=docker.service
After=docker.service network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/TaxionBack
ExecStart=/usr/bin/docker compose -f docker-compose.prod.yml --env-file .env.production.local up -d
ExecStop=/usr/bin/docker compose -f docker-compose.prod.yml down
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
```

**Активация сервиса:**

```bash
sudo systemctl daemon-reload
sudo systemctl enable taxion.service
sudo systemctl start taxion.service
sudo systemctl status taxion.service
```

### Шаг 10: Финальная проверка (5 минут)

```bash
# 1. Проверяем доступность через HTTPS
curl https://api.taxion.ru/health
curl https://api.taxion.ru/health/services

# 2. Проверяем логи
docker compose -f docker-compose.prod.yml logs --tail 50

# 3. Проверяем использование ресурсов
docker stats --no-stream

# 4. Проверяем диск
df -h
docker system df

# 5. Первый вход в систему
# Открыть в браузере: https://api.taxion.ru
# Войти как super admin с данными из .env.production.local
# СРАЗУ СМЕНИТЬ ПАРОЛЬ!
```

---

## VIII. ПОСЛЕ ДЕПЛОЯ (КРИТИЧЕСКИ ВАЖНО!)

- [ ] **Сменить пароль суперадмина** сразу после первого входа!
- [ ] Проверить отправку email (создать тестового пользователя и отправить приглашение)
- [ ] Проверить работу WebSocket (открыть чат)
- [ ] Настроить мониторинг (логи, метрики)
- [ ] Сделать первый ручной backup
- [ ] Записать все пароли в безопасное место (password manager)
- [ ] Удалить .env.production.local с локальной машины (если копировали)
- [ ] Настроить регулярные обновления системы
- [ ] Документировать все изменения

---

## IX. SECURITY CHECKLIST

- [ ] Все пароли по умолчанию изменены
- [ ] SSL сертификат установлен и работает
- [ ] HSTS заголовки включены
- [ ] Firewall настроен (только 22, 80, 443 открыты)
- [ ] SSH вход только по ключу (отключить пароли)
- [ ] Fail2ban установлен (опционально)
- [ ] Автоматический backup настроен
- [ ] Логи ротируются (Docker logging configured)
- [ ] Ресурсы контейнеров ограничены (уже в docker-compose.prod.yml)
- [ ] PostgreSQL и Redis не доступны извне (только внутри Docker сети)
- [ ] CORS настроен только на нужные домены
- [ ] Пароль суперадмина изменен после первого входа

---

## X. ПОЛЕЗНЫЕ КОМАНДЫ

### Просмотр логов

```bash
# Все сервисы
docker compose -f docker-compose.prod.yml logs -f

# Конкретный сервис
docker compose -f docker-compose.prod.yml logs -f gateway

# Последние 100 строк
docker logs tachyon-user-service-prod --tail 100
```

### Перезапуск сервисов

```bash
# Все сервисы
docker compose -f docker-compose.prod.yml restart

# Конкретный сервис
docker compose -f docker-compose.prod.yml restart gateway
```

### Обновление

```bash
cd /opt/TaxionBack
git pull origin main
docker compose -f docker-compose.prod.yml up -d --build
```

### Backup вручную

```bash
/opt/TaxionBack/backup.sh
```

### Мониторинг ресурсов

```bash
docker stats
htop
df -h
```

### Проверка health

```bash
curl https://api.taxion.ru/health/services
```

### Подключение к БД

```bash
# PostgreSQL
docker exec -it tachyon-postgres-prod psql -U tachyon_user -d tachyon_messenger

# Redis
docker exec -it tachyon-redis-prod redis-cli -a "ВАШ_REDIS_PASSWORD"

# Просмотр активных сессий
docker exec -it tachyon-redis-prod redis-cli -a "ВАШ_REDIS_PASSWORD" KEYS "session:*"
```

---

## XI. TROUBLESHOOTING

### Проблема: Сервис не стартует

```bash
docker logs tachyon-SERVICE-NAME-prod
docker compose -f docker-compose.prod.yml restart SERVICE-NAME
```

### Проблема: 502 Bad Gateway

```bash
# Проверить gateway
docker logs tachyon-gateway-prod

# Проверить nginx
sudo nginx -t
sudo systemctl status nginx
```

### Проблема: База данных недоступна

```bash
docker exec -it tachyon-postgres-prod pg_isready -U tachyon_user
docker logs tachyon-postgres-prod
```

### Проблема: Email не отправляются

```bash
docker logs tachyon-notification-service-prod | grep -i smtp

# Проверить SMTP_* переменные в .env
docker compose -f docker-compose.prod.yml exec notification-service printenv | grep SMTP
```

### Проблема: Нехватка памяти

```bash
# Проверяем использование памяти
docker stats

# Очищаем неиспользуемые образы и контейнеры
docker system prune -a
```

### Проблема: WebSocket не работает

```bash
# Проверить Nginx конфигурацию для WebSocket
sudo nginx -t

# Проверить логи chat-service
docker logs tachyon-chat-service-prod

# Проверить что location /ws настроен правильно в Nginx
```

---

## XII. КОНТАКТЫ И ПОДДЕРЖКА

- **Репозиторий**: https://github.com/mishkajackson/Taxion
- **Документация**: `/opt/TaxionBack/README.md`
- **Production Guide**: `/opt/TaxionBack/docs/PRODUCTION_DEPLOYMENT.md`
- **Quick Start**: `/opt/TaxionBack/docs/PRODUCTION_QUICKSTART.md`

---

## XIII. ПРИЛОЖЕНИЯ

### Приложение A: Пример Nginx конфигурации

Полная конфигурация находится в [README.md](README.md), секция "Настройка Nginx".

### Приложение B: Backup скрипт

Полный скрипт находится в [docs/PRODUCTION_DEPLOYMENT.md](docs/PRODUCTION_DEPLOYMENT.md), секция "Backup".

### Приложение C: Системные требования по ресурсам

| Компонент | CPU | RAM | Диск |
|-----------|-----|-----|------|
| PostgreSQL | 1-2 ядра | 1-2 GB | 10+ GB |
| Redis | 0.5-1 ядро | 512MB-1GB | 1+ GB |
| Gateway | 0.5-2 ядра | 512MB-1GB | 100 MB |
| User Service | 0.25-1 ядро | 256-512 MB | 100 MB |
| Chat Service | 0.25-1 ядро | 256-512 MB | 100 MB |
| Task Service | 0.25-1 ядро | 256-512 MB | 100 MB |
| Calendar Service | 0.1-0.5 ядра | 128-256 MB | 100 MB |
| Poll Service | 0.1-0.5 ядра | 128-256 MB | 100 MB |
| Notification Service | 0.25-1 ядро | 256-512 MB | 100 MB |
| File Service | 0.25-1 ядро | 256MB-1GB | 10+ GB |
| **ИТОГО** | **4+ ядер** | **8+ GB** | **50+ GB** |

---

**Конец документа**
