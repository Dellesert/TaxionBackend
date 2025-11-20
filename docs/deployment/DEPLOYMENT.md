# Deployment Guide - Tachyon Messenger

Руководство по деплою Tachyon Messenger с использованием Docker и GitHub Container Registry.

## Содержание

- [Обзор](#обзор)
- [Требования](#требования)
- [Настройка GitHub Container Registry](#настройка-github-container-registry)
- [Автоматическая сборка образов](#автоматическая-сборка-образов)
- [Деплой на production](#деплой-на-production)
- [Управление версиями](#управление-версиями)
- [Откат версий](#откат-версий)
- [Мониторинг и логи](#мониторинг-и-логи)

## Обзор

Проект использует:
- **GitHub Container Registry (ghcr.io)** - бесплатное приватное хранилище Docker образов
- **GitHub Actions** - автоматическая сборка и публикация образов
- **Docker Compose** - оркестрация контейнеров
- **Semantic Versioning** - версионирование образов

### Архитектура деплоя

```
GitHub Repository
    ↓
GitHub Actions (CI/CD)
    ↓
GitHub Container Registry (ghcr.io)
    ↓
Production Server
    ↓
Docker Compose → Containers
```

## Требования

### Локальные требования

- Docker Engine 20.10+
- Docker Compose v2.0+
- Git
- Bash (для скриптов деплоя)

### Серверные требования (Production)

- Linux Server (Ubuntu 20.04+ рекомендуется)
- Docker Engine 20.10+
- Docker Compose v2.0+
- Минимум 4GB RAM
- Минимум 20GB свободного места на диске
- Открытый порт 8080 (или настроенный в .env)

## Настройка GitHub Container Registry

### 1. Настройка репозитория

GitHub Container Registry автоматически включен для всех репозиториев. Образы публикуются через GitHub Actions.

### 2. Создание Personal Access Token (PAT)

Для локального доступа к приватным образам:

1. Перейдите в [GitHub Settings > Developer settings > Personal access tokens](https://github.com/settings/tokens)
2. Нажмите **Generate new token (classic)**
3. Выберите scopes:
   - `read:packages` - для pull образов
   - `write:packages` - для push образов (только для CI/CD)
   - `delete:packages` - для удаления образов
4. Сохраните токен в безопасном месте

### 3. Логин в GitHub Container Registry

На сервере выполните:

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

Где:
- `GITHUB_TOKEN` - ваш Personal Access Token
- `YOUR_GITHUB_USERNAME` - ваше имя пользователя GitHub

### 4. Настройка .env.versions

Скопируйте и настройте файл версий:

```bash
cp .env.versions.example .env.versions
```

Отредактируйте `.env.versions`:

```env
REGISTRY=ghcr.io
GITHUB_REPOSITORY_OWNER=your-github-username

# Service versions
USER_SERVICE_VERSION=latest
CHAT_SERVICE_VERSION=latest
# ... и т.д.
```

## Автоматическая сборка образов

### Триггеры сборки

GitHub Actions автоматически собирает и публикует образы при:

1. **Push в main** - создает образы с тегами `main` и `latest`
2. **Push в develop** - создает образы с тегом `develop`
3. **Создание тега** (v*.*.*) - создает образы с версионными тегами
4. **Pull Request** - собирает образы для тестирования (не публикует)
5. **Manual trigger** - ручной запуск с кастомной версией

### Теги образов

Для каждого сервиса создаются следующие теги:

- `latest` - последняя версия из main
- `main` - последний коммит в main
- `develop` - последний коммит в develop
- `v1.2.3` - semantic version tag
- `v1.2` - major.minor version
- `v1` - major version only
- `main-abc123def` - коммит SHA из main

### Создание релиза с версией

1. Обновите версии в коде (если нужно)
2. Создайте и запушьте тег:

```bash
# Создание тега
git tag -a v1.2.3 -m "Release version 1.2.3"

# Отправка тега в GitHub
git push origin v1.2.3
```

3. GitHub Actions автоматически:
   - Соберет все образы
   - Опубликует их с тегом `v1.2.3`
   - Создаст дополнительные теги `v1.2` и `v1`

### Мануальный запуск сборки

1. Откройте GitHub Actions в вашем репозитории
2. Выберите workflow "Build and Push Docker Images"
3. Нажмите "Run workflow"
4. Укажите версию (опционально)

## Деплой на production

### Подготовка сервера

1. Клонируйте репозиторий:

```bash
git clone https://github.com/YOUR_USERNAME/TaxionBack.git
cd TaxionBack
```

2. Создайте production конфигурацию:

```bash
# Скопируйте шаблоны
cp .env.example .env.production
cp .env.versions.example .env.versions

# Отредактируйте файлы
nano .env.production  # Настройте production переменные
nano .env.versions    # Укажите версии сервисов
```

3. Настройте переменные в `.env.production`:

```env
# Database
POSTGRES_DB=tachyon_prod
POSTGRES_USER=tachyon_prod_user
POSTGRES_PASSWORD=strong_password_here
POSTGRES_PORT=5432

# Redis
REDIS_PASSWORD=strong_redis_password

# JWT
JWT_SECRET=your_super_secret_jwt_key_here
JWT_EXPIRY=24h

# Backend URL
BACKEND_URL=https://your-domain.com

# CORS
CORS_ORIGINS=https://your-frontend-domain.com

# ... остальные переменные
```

### Деплой с помощью скрипта

Простейший способ - использовать скрипт деплоя:

```bash
# Сделать скрипт исполняемым
chmod +x scripts/deployment/deploy.sh

# Запустить деплой
./scripts/deployment/deploy.sh deploy
```

Скрипт автоматически:
- Проверит все требования
- Подтянет последние образы
- Запустит сервисы
- Проверит здоровье всех сервисов
- Покажет статус

### Ручной деплой

Если предпочитаете ручное управление:

```bash
# 1. Логин в GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin

# 2. Pull образов
docker compose -f docker-compose.prod.yml \
  --env-file .env.production \
  --env-file .env.versions \
  pull

# 3. Запуск сервисов
docker compose -f docker-compose.prod.yml \
  --env-file .env.production \
  --env-file .env.versions \
  up -d

# 4. Проверка статуса
docker compose -f docker-compose.prod.yml ps

# 5. Проверка логов
docker compose -f docker-compose.prod.yml logs -f
```

## Управление версиями

### Стратегия версионирования

1. **Development** - тег `latest` или `develop`
2. **Staging** - тег `main` или pre-release версия
3. **Production** - конкретные версии (v1.2.3)

### Обновление одного сервиса

Если нужно обновить только один сервис:

1. Отредактируйте `.env.versions`:

```env
# Обновляем только chat-service до новой версии
CHAT_SERVICE_VERSION=v1.3.0
# Остальные сервисы остаются на прежних версиях
USER_SERVICE_VERSION=v1.2.3
TASK_SERVICE_VERSION=v1.2.3
# ...
```

2. Перезапустите только этот сервис:

```bash
# Pull нового образа
docker compose -f docker-compose.prod.yml pull chat-service

# Перезапуск сервиса
docker compose -f docker-compose.prod.yml up -d chat-service

# Или используйте скрипт
./scripts/deployment/deploy.sh restart chat-service
```

### Обновление всех сервисов

Для обновления всех сервисов до новых версий:

1. Обновите `.env.versions` со всеми новыми версиями
2. Запустите деплой:

```bash
./scripts/deployment/deploy.sh deploy
```

## Откат версий

### Быстрый откат сервиса

Если после обновления возникли проблемы:

1. Откройте `.env.versions`
2. Измените версию проблемного сервиса на предыдущую:

```env
# Откат с v1.3.0 на v1.2.3
CHAT_SERVICE_VERSION=v1.2.3
```

3. Перезапустите сервис:

```bash
./scripts/deployment/deploy.sh restart chat-service
```

### Полный откат всей системы

Если нужно откатить все сервисы:

1. Найдите backup вашего `.env.versions`:

```bash
# Восстановите из backup
cp .env.versions.backup .env.versions
```

2. Запустите повторный деплой:

```bash
./scripts/deployment/deploy.sh deploy
```

### Blue-Green Deployment

Для минимизации downtime используйте Blue-Green подход:

```bash
# 1. Поднимите новую версию с другим именем
docker compose -f docker-compose.prod.yml \
  -p tachyon-green \
  up -d

# 2. Проверьте работоспособность
./scripts/deployment/deploy.sh health

# 3. Переключите трафик (настройте nginx/load balancer)

# 4. Остановите старую версию
docker compose -f docker-compose.prod.yml \
  -p tachyon-blue \
  down
```

## Мониторинг и логи

### Просмотр статуса

```bash
# Статус всех сервисов
docker compose -f docker-compose.prod.yml ps

# Или используйте скрипт
./scripts/deployment/deploy.sh status
```

### Просмотр логов

```bash
# Все логи в режиме follow
docker compose -f docker-compose.prod.yml logs -f

# Логи конкретного сервиса
docker compose -f docker-compose.prod.yml logs -f chat-service

# Последние 100 строк
docker compose -f docker-compose.prod.yml logs --tail=100 chat-service

# Или используйте скрипт
./scripts/deployment/deploy.sh logs chat-service
```

### Health Checks

Проверка здоровья всех сервисов:

```bash
./scripts/deployment/deploy.sh health
```

Или вручную для каждого сервиса:

```bash
# Gateway
curl http://localhost:8080/health

# User Service (внутри контейнера)
docker exec tachyon-user-service-prod wget --no-verbose --tries=1 --spider http://localhost:8081/health
```

### Использование ресурсов

```bash
# Статистика использования ресурсов
docker stats

# Размер образов
docker images | grep tachyon

# Использование дискового пространства
docker system df
```

## Утилиты управления

### Скрипт deploy.sh

Основные команды:

```bash
# Полный деплой
./scripts/deployment/deploy.sh deploy

# Только pull образов
./scripts/deployment/deploy.sh pull

# Статус сервисов
./scripts/deployment/deploy.sh status

# Health check
./scripts/deployment/deploy.sh health

# Просмотр логов
./scripts/deployment/deploy.sh logs [service]

# Остановка всех сервисов
./scripts/deployment/deploy.sh down

# Перезапуск сервисов
./scripts/deployment/deploy.sh restart [service]
```

## Best Practices

### 1. Версионирование

- Используйте semantic versioning (v1.2.3)
- Для production всегда указывайте конкретные версии
- Не используйте `latest` в production

### 2. Безопасность

- Храните `.env.production` вне git
- Используйте сильные пароли
- Регулярно обновляйте токены доступа
- Ограничивайте права доступа к серверу

### 3. Мониторинг

- Регулярно проверяйте логи
- Настройте алерты на критические ошибки
- Мониторьте использование ресурсов

### 4. Backup

- Регулярно делайте backup базы данных
- Сохраняйте копии .env файлов
- Документируйте изменения версий

### 5. Обновления

- Тестируйте обновления на staging перед production
- Делайте backup перед обновлением
- Обновляйте по одному сервису за раз
- Держите план отката наготове

## Troubleshooting

### Проблема: Образы не pull'ятся

```bash
# Решение: Проверьте логин в GHCR
docker logout ghcr.io
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

### Проблема: Сервис не стартует

```bash
# Проверьте логи
docker compose -f docker-compose.prod.yml logs service-name

# Проверьте переменные окружения
docker compose -f docker-compose.prod.yml config
```

### Проблема: Health check fails

```bash
# Проверьте, что сервис слушает на правильном порту
docker exec container-name netstat -tlnp

# Проверьте зависимости
docker compose -f docker-compose.prod.yml ps
```

### Проблема: Нехватка ресурсов

```bash
# Очистите неиспользуемые ресурсы
docker system prune -a

# Удалите старые образы
docker images | grep tachyon | grep -v latest | awk '{print $3}' | xargs docker rmi
```

## Дополнительные ресурсы

- [Docker Documentation](https://docs.docker.com/)
- [GitHub Container Registry Docs](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)

## Поддержка

Если у вас возникли проблемы:

1. Проверьте логи сервисов
2. Просмотрите Issues в репозитории
3. Создайте новый Issue с описанием проблемы
