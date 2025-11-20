# Quick Deployment Guide

Быстрое руководство по деплою Tachyon Messenger на production сервер.

## 1. Подготовка (5 минут)

### На GitHub - создание токена

1. Получите Personal Access Token (PAT):
   - Откройте [GitHub Settings → Tokens](https://github.com/settings/tokens)
   - Нажмите **"Generate new token (classic)"**
   - Выберите права: ✅ `read:packages` (обязательно)
   - Сохраните токен (показывается один раз!)

Токен выглядит так: `ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

📖 Детальная инструкция: [GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md)

### На сервере

```bash
# Установка Docker (если не установлен)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Клонирование репозитория
git clone https://github.com/YOUR_USERNAME/TaxionBack.git
cd TaxionBack
```

## 2. Настройка docker-compose.prod.yml (5 минут)

Откройте файл и замените `YOUR_GITHUB_USERNAME` на ваше имя пользователя:

```bash
nano docker-compose.prod.yml
```

Найдите и измените все строки с `image:`:
```yaml
# Было:
image: ghcr.io/YOUR_GITHUB_USERNAME/tachyon-user:latest

# Станет (замените на ваш username):
image: ghcr.io/myusername/tachyon-user:latest
```

**Или автоматически:**
```bash
# Замените YOUR_USERNAME на ваш GitHub username
sed -i 's/YOUR_GITHUB_USERNAME/myusername/g' docker-compose.prod.yml
```

## 3. Настройка .env.production (5 минут)

```bash
# Создание конфигурационного файла
cp .env.example .env.production

# Редактирование
nano .env.production
```

**Минимальные настройки:**
```env
# Database
POSTGRES_PASSWORD=your_secure_password_here
REDIS_PASSWORD=your_redis_password_here

# JWT
JWT_SECRET=your_super_secret_jwt_key

# URLs
BACKEND_URL=http://your-server-ip:8080
CORS_ORIGINS=http://your-frontend-domain.com
```

## 4. Настройка авторизации GHCR (5 минут)

### Вариант А: Файл с credentials (рекомендуется) ⭐

```bash
# Создайте файл с credentials
cat > ~/.github-credentials << 'EOF'
GITHUB_TOKEN=ghp_your_actual_token_here
GITHUB_USERNAME=your-github-username
EOF

# Ограничьте права
chmod 600 ~/.github-credentials
```

Скрипт деплоя автоматически использует этот файл!

### Вариант Б: Переменные окружения

```bash
# Установите переменные
export GITHUB_TOKEN="ghp_your_token"
export GITHUB_USERNAME="your-username"

# Залогиньтесь
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

📖 Другие способы: [GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md)

## 5. Деплой (1 минута)

```bash
# Если используете файл credentials (Вариант А)
./scripts/deployment/deploy.sh deploy

# Или вручную
docker compose -f docker-compose.prod.yml --env-file .env.production up -d
```

## 6. Проверка

```bash
# Проверка авторизации
docker info | grep ghcr.io  # Должен показать ghcr.io в списке

# Проверка статуса
docker compose -f docker-compose.prod.yml ps

# Проверка логов
docker compose -f docker-compose.prod.yml logs -f

# Проверка health
curl http://localhost:8080/health
```

---

## Управление версиями

### Изменение версии одного сервиса

Откройте [docker-compose.prod.yml](docker-compose.prod.yml) и измените версию:

```yaml
# Было:
image: ghcr.io/myusername/tachyon-chat:latest

# Станет:
image: ghcr.io/myusername/tachyon-chat:v1.1.0
```

Перезапустите сервис:
```bash
docker compose -f docker-compose.prod.yml pull chat-service
docker compose -f docker-compose.prod.yml up -d chat-service
```

### Обновление всех сервисов до новой версии

1. Откройте [docker-compose.prod.yml](docker-compose.prod.yml)
2. Измените `:latest` на `:v1.1.0` для всех сервисов:

```bash
# Автоматически для всех сервисов
sed -i 's/:latest/:v1.1.0/g' docker-compose.prod.yml
```

3. Обновите:
```bash
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### Откат на предыдущую версию

1. Измените версии обратно в [docker-compose.prod.yml](docker-compose.prod.yml)
2. Перезапустите:

```bash
# Изменить :v1.1.0 обратно на :v1.0.0
sed -i 's/:v1.1.0/:v1.0.0/g' docker-compose.prod.yml

# Перезапустить
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

**Или используйте git:**
```bash
# Откатить файл на предыдущую версию
git checkout HEAD~1 docker-compose.prod.yml

# Перезапустить
docker compose -f docker-compose.prod.yml up -d
```

---

## Автоматическая сборка образов

### При push в main или develop

GitHub Actions автоматически соберет образы с тегом `main` или `develop`:

```bash
git push origin main
# Через 5-10 минут образы будут доступны как :main
```

### Создание релиза с версией

```bash
# Создать тег версии
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Через 5-10 минут образы будут доступны как :v1.0.0
```

GitHub Actions создаст образы с тегами:
- `v1.0.0` - точная версия
- `v1.0` - минорная версия
- `v1` - мажорная версия
- `latest` - если это main ветка

---

## Полезные команды

### Просмотр логов

```bash
# Все сервисы
docker compose -f docker-compose.prod.yml logs -f

# Конкретный сервис
docker compose -f docker-compose.prod.yml logs -f chat-service

# Последние 100 строк
docker compose -f docker-compose.prod.yml logs --tail=100 chat-service
```

### Перезапуск сервисов

```bash
# Перезапуск конкретного сервиса
docker compose -f docker-compose.prod.yml restart chat-service

# Перезапуск всех сервисов
docker compose -f docker-compose.prod.yml restart
```

### Остановка

```bash
# Остановить все сервисы
docker compose -f docker-compose.prod.yml down

# Остановить и удалить volumes (ВНИМАНИЕ: удалит данные!)
docker compose -f docker-compose.prod.yml down -v
```

### Обновление образов

```bash
# Pull новых образов
docker compose -f docker-compose.prod.yml pull

# Пересоздать контейнеры с новыми образами
docker compose -f docker-compose.prod.yml up -d --force-recreate
```

### Очистка

```bash
# Удалить старые образы
docker image prune -a

# Полная очистка
docker system prune -a
```

---

## Troubleshooting

### Не pull'ятся образы

```bash
# Проверьте логин
docker login ghcr.io

# Залогиньтесь снова
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

### Сервис не стартует

```bash
# Смотрите логи
docker compose -f docker-compose.prod.yml logs service-name

# Проверьте .env файл
cat .env.production
```

### Health check fails

```bash
# Подождите 1-2 минуты после старта
sleep 60

# Проверьте зависимости (БД, Redis)
docker compose -f docker-compose.prod.yml ps
```

### Ошибки прав доступа

```bash
# Создайте нужные директории
mkdir -p logs backups

# Дайте права
chmod -R 777 logs backups uploads
```

---

## Управление версиями в Git

### Коммит текущей версии

```bash
# Коммит конфигурации
git add docker-compose.prod.yml
git commit -m "Production: Update to v1.1.0"
git push
```

### История изменений

```bash
# Посмотреть историю изменений docker-compose
git log -p docker-compose.prod.yml

# Посмотреть разницу с предыдущей версией
git diff HEAD~1 docker-compose.prod.yml
```

### Откат через git

```bash
# Посмотреть предыдущие версии
git log --oneline docker-compose.prod.yml

# Откатить на конкретный коммит
git checkout <commit-hash> docker-compose.prod.yml

# Применить
docker compose -f docker-compose.prod.yml up -d
```

---

## Workflow для production

### Обычное обновление

```bash
# 1. На dev машине - создайте релиз
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0

# 2. Дождитесь сборки в GitHub Actions (5-10 минут)

# 3. На production сервере - обновите версии
nano docker-compose.prod.yml  # Измените :latest на :v1.1.0

# 4. Деплой
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d

# 5. Проверка
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f

# 6. Если все ок - коммит
git add docker-compose.prod.yml
git commit -m "Production: Deploy v1.1.0"
git push
```

### Экстренный откат

```bash
# 1. Откатить через git
git checkout HEAD~1 docker-compose.prod.yml

# 2. Применить
docker compose -f docker-compose.prod.yml up -d --force-recreate

# 3. Проверка
docker compose -f docker-compose.prod.yml logs -f
```

---

## Архитектура

```
┌─────────────────────────────────────────┐
│        GitHub Repository                │
├─────────────────────────────────────────┤
│  - Push code                            │
│  - Create tag (v1.0.0)                  │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│       GitHub Actions (CI/CD)            │
├─────────────────────────────────────────┤
│  - Build Docker images                  │
│  - Push to GHCR (ghcr.io)              │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│  GitHub Container Registry (ghcr.io)    │
├─────────────────────────────────────────┤
│  ghcr.io/username/tachyon-user:v1.0.0  │
│  ghcr.io/username/tachyon-chat:v1.0.0  │
│  ghcr.io/username/tachyon-task:v1.0.0  │
│  ...                                    │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│       Production Server                 │
├─────────────────────────────────────────┤
│  docker compose -f                      │
│    docker-compose.prod.yml up -d        │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│         Running Services                │
├─────────────────────────────────────────┤
│  - Gateway (8080)                       │
│  - User Service (8081)                  │
│  - Chat Service (8082)                  │
│  - Task Service (8083)                  │
│  - And more...                          │
└─────────────────────────────────────────┘
```

---

## Дополнительные ресурсы

- [DEPLOYMENT.md](DEPLOYMENT.md) - детальная документация
- [DOCKER_VERSIONING.md](DOCKER_VERSIONING.md) - про версионирование и GHCR
- [GitHub Actions Workflow](.github/workflows/docker-publish.yml) - CI/CD конфигурация

## Преимущества

✅ **Бесплатно** - GHCR не требует оплаты
✅ **Приватность** - образы доступны только вам
✅ **Простота** - версии прямо в docker-compose.yml
✅ **История** - все версии в git
✅ **Откат** - через git или просто изменить версию
✅ **Автоматизация** - GitHub Actions собирает автоматически

## Безопасность

- 🔒 Приватные образы в GHCR
- 🔑 Токены с ограниченными правами
- 🛡️ .env.production не в git
- 🔐 Сильные пароли для БД
