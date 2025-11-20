# Docker Versioning & GitHub Container Registry Setup

## Что было настроено

### 1. GitHub Container Registry (GHCR)
- ✅ Бесплатное приватное хранилище Docker образов
- ✅ Автоматическая публикация через GitHub Actions
- ✅ Поддержка версионирования (semantic versioning)

### 2. GitHub Actions Workflow
- **Файл**: [.github/workflows/docker-publish.yml](.github/workflows/docker-publish.yml)
- **Триггеры**:
  - Push в `main` → `latest` и `main` теги
  - Push в `develop` → `develop` тег
  - Создание тега `v*.*.*` → версионный тег (v1.2.3)
  - Manual trigger с кастомной версией

### 3. Docker Compose Production
- **Файл**: [docker-compose.prod.yml](docker-compose.prod.yml)
- Все сервисы используют образы из GHCR
- Версии задаются через переменные окружения

### 4. Конфигурация версий
- **Файл**: [.env.versions.example](.env.versions.example)
- Управление версиями каждого сервиса
- Независимое обновление сервисов

### 5. Скрипты деплоя
- **Linux/Mac**: [scripts/deployment/deploy.sh](scripts/deployment/deploy.sh)
- **Windows**: [scripts/deployment/deploy.ps1](scripts/deployment/deploy.ps1)
- Автоматизация деплоя и управления

## Быстрый старт

### Шаг 1: Получить GitHub Token

1. Откройте [GitHub Settings → Tokens](https://github.com/settings/tokens)
2. Создайте token с правами `read:packages`
3. Сохраните токен

### Шаг 2: Настроить GitHub Actions

**ВАЖНО**: Обновите `.github/workflows/docker-publish.yml`:

```yaml
env:
  REGISTRY: ghcr.io
  IMAGE_PREFIX: ${{ github.repository_owner }}/tachyon  # автоматически подставится ваш username
```

Никаких дополнительных secrets не нужно - GitHub Actions использует встроенный `GITHUB_TOKEN`.

### Шаг 3: Первая сборка

```bash
# Создайте первый релиз
git tag -a v1.0.0 -m "Initial release"
git push origin v1.0.0
```

GitHub Actions автоматически:
- Соберет все 10 сервисов
- Опубликует в `ghcr.io/YOUR_USERNAME/tachyon-*:v1.0.0`
- Создаст теги `v1.0`, `v1`, `latest`

### Шаг 4: Деплой на сервер

```bash
# Клонируйте репозиторий
git clone https://github.com/YOUR_USERNAME/TaxionBack.git
cd TaxionBack

# Настройте конфигурацию
cp .env.versions.example .env.versions
nano .env.versions  # Укажите GITHUB_REPOSITORY_OWNER=your-username

cp .env.example .env.production
nano .env.production  # Настройте production параметры

# Логин в GHCR
echo YOUR_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin

# Деплой
chmod +x scripts/deployment/deploy.sh
./scripts/deployment/deploy.sh deploy
```

## Структура образов

После сборки в GHCR будут доступны образы:

```
ghcr.io/YOUR_USERNAME/tachyon-user:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-chat:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-task:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-file:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-calendar:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-poll:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-notification:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-analytics:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-gateway:v1.0.0
ghcr.io/YOUR_USERNAME/tachyon-backup:v1.0.0
```

## Управление версиями

### Создание новой версии

```bash
# Внесите изменения в код
git add .
git commit -m "Add new feature"
git push origin main

# Создайте новый релиз
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
```

### Обновление конкретного сервиса

Если изменился только один сервис (например, chat):

1. На сервере обновите `.env.versions`:
```env
CHAT_SERVICE_VERSION=v1.1.0  # Новая версия
USER_SERVICE_VERSION=v1.0.0  # Старая версия
# ... остальные на старых версиях
```

2. Перезапустите только этот сервис:
```bash
./scripts/deployment/deploy.sh restart chat-service
```

### Откат версии

```bash
# В .env.versions укажите старую версию
nano .env.versions
# CHAT_SERVICE_VERSION=v1.0.0  # Откат с v1.1.0

# Перезапустите
./scripts/deployment/deploy.sh restart chat-service
```

## Стратегии деплоя

### 1. Development
```env
# .env.versions
USER_SERVICE_VERSION=latest
CHAT_SERVICE_VERSION=develop
```

### 2. Staging
```env
# .env.versions
USER_SERVICE_VERSION=main
CHAT_SERVICE_VERSION=main
```

### 3. Production
```env
# .env.versions
USER_SERVICE_VERSION=v1.0.0
CHAT_SERVICE_VERSION=v1.0.0
```

## Примеры использования

### Просмотр доступных версий

На GitHub:
1. Перейдите в ваш репозиторий
2. Packages → выберите сервис
3. Увидите все доступные теги

### Pull образа локально

```bash
docker pull ghcr.io/YOUR_USERNAME/tachyon-chat:v1.0.0
```

### Запуск конкретной версии

```bash
docker run -d \
  --name chat-test \
  -p 8082:8082 \
  --env-file .env.production \
  ghcr.io/YOUR_USERNAME/tachyon-chat:v1.0.0
```

## Автоматизация

### Auto-deploy при push в main

Добавьте в `.github/workflows/docker-publish.yml` в конце:

```yaml
  deploy:
    needs: build-and-push
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy to production
        uses: appleboy/ssh-action@master
        with:
          host: ${{ secrets.PROD_HOST }}
          username: ${{ secrets.PROD_USER }}
          key: ${{ secrets.PROD_SSH_KEY }}
          script: |
            cd /path/to/TaxionBack
            git pull
            ./scripts/deployment/deploy.sh deploy
```

### Scheduled updates (еженедельно)

```yaml
on:
  schedule:
    - cron: '0 2 * * 0'  # Каждое воскресенье в 2:00
```

## Мониторинг

### Посмотреть какие версии запущены

```bash
docker compose -f docker-compose.prod.yml ps --format json | jq '.[] | {name: .Name, image: .Image}'
```

### Проверить размер образов

```bash
docker images | grep tachyon
```

### Очистка старых образов

```bash
# Удалить образы старше 30 дней
docker image prune -a --filter "until=720h"
```

## Безопасность

### Best Practices

1. **Токены**:
   - Используйте токены с минимальными правами
   - Регулярно обновляйте токены
   - Не храните токены в git

2. **Образы**:
   - Делайте образы приватными
   - Используйте multi-stage builds
   - Регулярно обновляйте базовые образы

3. **Версии**:
   - Используйте конкретные версии в production
   - Тестируйте на staging перед production
   - Документируйте изменения

### Настройка приватности пакетов

1. Откройте Package в GitHub
2. Package settings → Danger Zone
3. Change package visibility → Private

## Troubleshooting

### Error: authentication required

```bash
# Re-login
docker logout ghcr.io
echo YOUR_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

### Error: requested access to the resource is denied

Проверьте:
1. Токен имеет права `read:packages`
2. Пакет настроен как приватный и вы имеете доступ
3. Username написан правильно

### Образы не обновляются

```bash
# Force pull
docker compose -f docker-compose.prod.yml pull --no-cache
docker compose -f docker-compose.prod.yml up -d --force-recreate
```

## Полезные команды

```bash
# Логин в GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin

# Pull всех образов
docker compose -f docker-compose.prod.yml pull

# Запуск с конкретными версиями
docker compose -f docker-compose.prod.yml \
  --env-file .env.production \
  --env-file .env.versions \
  up -d

# Просмотр логов
docker compose -f docker-compose.prod.yml logs -f

# Остановка
docker compose -f docker-compose.prod.yml down

# Полная очистка
docker compose -f docker-compose.prod.yml down -v
docker system prune -a
```

## Дополнительные ресурсы

- 📖 [Подробная документация](DEPLOYMENT.md)
- 🚀 [Быстрый старт](QUICK_DEPLOY.md)
- 🐳 [Docker Compose файл](docker-compose.prod.yml)
- ⚙️ [GitHub Actions Workflow](.github/workflows/docker-publish.yml)
- 🔧 [Скрипт деплоя](scripts/deployment/deploy.sh)

## FAQ

**Q: Сколько это стоит?**
A: Бесплатно! GHCR бесплатен для публичных и приватных репозиториев.

**Q: Есть ли лимиты на хранение?**
A: Да, 500MB для бесплатного аккаунта, 2GB для GitHub Pro.

**Q: Можно ли использовать с Docker Hub?**
A: Да, просто измените `REGISTRY` в workflow на `docker.io`.

**Q: Как обновить все сервисы разом?**
A: Создайте новый тег версии, все образы соберутся автоматически.

**Q: Нужно ли пересобирать все сервисы при изменении одного?**
A: Нет, можно пересобрать только изменённый, но обычно проще пересобрать все.

**Q: Как тестировать перед production?**
A: Используйте тег `develop` для staging окружения.
