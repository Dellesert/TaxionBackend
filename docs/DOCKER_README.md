# Docker Deployment - Quick Reference

Краткая справка по деплою Tachyon с Docker и GitHub Container Registry.

## Документация

| Документ | Описание |
|----------|----------|
| [QUICK_DEPLOY.md](QUICK_DEPLOY.md) | 🚀 Быстрый старт - деплой за 20 минут |
| [GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md) | 🔑 Настройка авторизации в GHCR |
| [DEPLOYMENT.md](DEPLOYMENT.md) | 📖 Детальное руководство по деплою |
| [DOCKER_VERSIONING.md](DOCKER_VERSIONING.md) | 📦 Версионирование и работа с GHCR |

## Быстрый старт

### 1. Создайте GitHub Token
```bash
# Откройте: https://github.com/settings/tokens
# Права: read:packages
```

### 2. Настройте credentials на сервере
```bash
cat > ~/.github-credentials << 'EOF'
GITHUB_TOKEN=ghp_your_token_here
GITHUB_USERNAME=your-username
EOF
chmod 600 ~/.github-credentials
```

### 3. Настройте docker-compose.prod.yml
```bash
# Замените YOUR_GITHUB_USERNAME на ваш username
sed -i 's/YOUR_GITHUB_USERNAME/myusername/g' docker-compose.prod.yml
```

### 4. Деплой
```bash
./scripts/deployment/deploy.sh deploy
```

## Основные команды

```bash
# Деплой
./scripts/deployment/deploy.sh deploy

# Статус
./scripts/deployment/deploy.sh status

# Логи
./scripts/deployment/deploy.sh logs [service]

# Перезапуск
./scripts/deployment/deploy.sh restart [service]

# Остановка
./scripts/deployment/deploy.sh down
```

## Управление версиями

Откройте [docker-compose.prod.yml](docker-compose.prod.yml) и измените версии:

```yaml
# Было:
image: ghcr.io/username/tachyon-chat:latest

# Станет:
image: ghcr.io/username/tachyon-chat:v1.1.0
```

Применить:
```bash
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

## Создание релиза

```bash
# 1. Создайте тег
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 2. GitHub Actions соберет образы (5-10 минут)

# 3. Обновите версии в docker-compose.prod.yml
sed -i 's/:latest/:v1.0.0/g' docker-compose.prod.yml

# 4. Деплой
./scripts/deployment/deploy.sh deploy
```

## Откат версии

```bash
# Через git
git checkout HEAD~1 docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d

# Или вручную измените версии в файле
sed -i 's/:v1.1.0/:v1.0.0/g' docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

## Архитектура

```
GitHub → GitHub Actions → GHCR → Production Server
                  ↓
         docker-compose.prod.yml
                  ↓
            Running Services
```

## Сервисы

- **Gateway** - порт 8080 (единственный открытый наружу)
- **User Service** - 8081
- **Chat Service** - 8082
- **Task Service** - 8083
- **Calendar Service** - 8084
- **Poll Service** - 8085
- **Analytics Service** - 8086
- **Notification Service** - 8087
- **File Service** - 8088
- **Backup Service** - 8089
- **PostgreSQL** - внутренний
- **Redis** - внутренний

## Troubleshooting

### Образы не pull'ятся
```bash
# Проверьте авторизацию
docker info | grep ghcr.io

# Залогиньтесь заново
source ~/.github-credentials
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

### Сервис не стартует
```bash
# Логи
docker compose -f docker-compose.prod.yml logs service-name

# Статус
docker compose -f docker-compose.prod.yml ps
```

### Health check fails
```bash
# Подождите 1-2 минуты
sleep 60 && curl http://localhost:8080/health
```

## Безопасность

✅ **Делайте:**
- Используйте файл `~/.github-credentials` с правами 600
- Храните `.env.production` вне git
- Регулярно обновляйте токены
- Используйте конкретные версии (v1.0.0) для production

❌ **Не делайте:**
- Не коммитьте токены в git
- Не используйте `latest` в production
- Не открывайте порты БД наружу

## Мониторинг

```bash
# Логи всех сервисов
docker compose -f docker-compose.prod.yml logs -f

# Статистика ресурсов
docker stats

# Health check
curl http://localhost:8080/health
```

## Полезные ссылки

- [GitHub Container Registry](https://ghcr.io)
- [GitHub Actions Workflow](.github/workflows/docker-publish.yml)
- [GitHub Packages Docs](https://docs.github.com/en/packages)
