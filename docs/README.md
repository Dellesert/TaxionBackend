# 📚 Документация Tachyon Messenger

Добро пожаловать в документацию Tachyon Messenger Backend!

## 🚀 Быстрый старт

**Первый раз деплоите проект?** Следуйте этим шагам:

### Шаг 1: Быстрый деплой
**→ [QUICK_DEPLOY.md](QUICK_DEPLOY.md)** - Разверните проект за 20 минут

### Шаг 2: Настройка авторизации
**→ [GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md)** - Настройте доступ к приватным образам

### Шаг 3: Справка по командам
**→ [DOCKER_README.md](DOCKER_README.md)** - Шпаргалка по основным командам

---

## 📁 Структура документации

### 🐳 Deployment - Развертывание

**Для начинающих:**
- **[QUICK_DEPLOY.md](QUICK_DEPLOY.md)** 🟢
  - Пошаговая инструкция для первого деплоя
  - Время: 20 минут
  - Уровень: Начинающий

- **[GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md)** 🟢
  - Настройка авторизации в GitHub Container Registry
  - 3 способа настройки (файл, переменные, ручной)
  - Troubleshooting авторизации

- **[DOCKER_README.md](DOCKER_README.md)** 🟢
  - Краткая справка по командам
  - Быстрый доступ к основным операциям
  - Шпаргалка

**Для продвинутых:**
- **[deployment/DEPLOYMENT.md](deployment/DEPLOYMENT.md)** 🟡
  - Детальное руководство по деплою
  - Настройка production окружения
  - Мониторинг и логи
  - Полный troubleshooting

- **[deployment/DOCKER_VERSIONING.md](deployment/DOCKER_VERSIONING.md)** 🟡
  - Управление версиями Docker образов
  - Работа с GitHub Container Registry
  - Стратегии версионирования
  - CI/CD настройка

---

### ⚙️ Configuration - Конфигурация

- **[configuration/ENV_CONFIGURATION.md](configuration/ENV_CONFIGURATION.md)**
  - Настройка переменных окружения
  - Описание всех .env параметров
  - Примеры конфигураций

- **[configuration/ENVIRONMENT_COMPARISON.md](configuration/ENVIRONMENT_COMPARISON.md)**
  - Сравнение Development vs Production
  - Рекомендации по настройке
  - Различия в конфигурации

---

### 📖 Guides - Руководства

- **[guides/FCM_PUSH_NOTIFICATIONS.md](guides/FCM_PUSH_NOTIFICATIONS.md)**
  - Настройка push-уведомлений через Firebase Cloud Messaging
  - Создание Firebase проекта
  - Интеграция с notification-service

---

### 🔌 API

- **[api/API_ERROR_RESPONSES.md](api/API_ERROR_RESPONSES.md)**
  - Справочник по ошибкам API
  - Коды ошибок и их значения
  - Примеры ответов

---

### 📦 Archived - Архив

Старые версии документации (для справки):
- [archived/PRODUCTION_DEPLOYMENT.md](archived/PRODUCTION_DEPLOYMENT.md)
- [archived/PRODUCTION_QUICKSTART.md](archived/PRODUCTION_QUICKSTART.md)
- [archived/DEPLOYMENT_CHECKLIST.md](archived/DEPLOYMENT_CHECKLIST.md)
- [archived/DEPLOYMENT_REQUIREMENTS.md](archived/DEPLOYMENT_REQUIREMENTS.md)

---

## 🎯 Сценарии использования

### 🟢 Сценарий 1: Первый деплой

```bash
# 1. Прочитайте QUICK_DEPLOY.md
# 2. Создайте GitHub Token
# 3. Настройте авторизацию
cat > ~/.github-credentials << 'EOF'
GITHUB_TOKEN=your_token
GITHUB_USERNAME=your_username
EOF
chmod 600 ~/.github-credentials

# 4. Деплой
./scripts/deployment/deploy.sh deploy
```

**Время:** ~30 минут | **Сложность:** 🟢 Легко

---

### 🟡 Сценарий 2: Обновление версии

```bash
# 1. Создайте релиз
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0

# 2. Обновите docker-compose.prod.yml
sed -i 's/:v1.0.0/:v1.1.0/g' docker-compose.prod.yml

# 3. Примените
./scripts/deployment/deploy.sh deploy
```

**Время:** ~15 минут | **Сложность:** 🟡 Средне

---

### 🔴 Сценарий 3: Откат версии

```bash
# Откат через git
git checkout HEAD~1 docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

**Время:** ~5 минут | **Сложность:** 🟢 Легко

---

## 🔍 Поиск по темам

### Deployment
- [Быстрый старт](QUICK_DEPLOY.md)
- [Полное руководство](deployment/DEPLOYMENT.md)
- [Версионирование](deployment/DOCKER_VERSIONING.md)

### Авторизация
- [Настройка GHCR](GITHUB_AUTH_SETUP.md)
- [Конфигурация](configuration/ENV_CONFIGURATION.md)

### Руководства
- [Push-уведомления](guides/FCM_PUSH_NOTIFICATIONS.md)
- [API ошибки](api/API_ERROR_RESPONSES.md)

---

## 🔧 Основные команды

```bash
# Деплой
./scripts/deployment/deploy.sh deploy

# Логи
./scripts/deployment/deploy.sh logs [service]

# Статус
./scripts/deployment/deploy.sh status

# Health check
./scripts/deployment/deploy.sh health

# Перезапуск
./scripts/deployment/deploy.sh restart [service]

# Остановка
./scripts/deployment/deploy.sh down
```

---

## 🆘 Troubleshooting

| Проблема | Решение |
|----------|---------|
| Образы не pull'ятся | [GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md) |
| Сервис не стартует | `docker compose logs service-name` |
| Health check fails | Подождите 1-2 минуты |

Подробнее: [deployment/DEPLOYMENT.md#troubleshooting](deployment/DEPLOYMENT.md#troubleshooting)

---

## 📊 Уровни сложности

| Уровень | Документы |
|---------|-----------|
| 🟢 Начинающий | QUICK_DEPLOY, GITHUB_AUTH_SETUP, DOCKER_README |
| 🟡 Средний | DEPLOYMENT, DOCKER_VERSIONING, FCM_PUSH_NOTIFICATIONS |
| 🔴 Продвинутый | ENV_CONFIGURATION, API_ERROR_RESPONSES |

---

## 🎓 Рекомендуемый порядок изучения

1. [QUICK_DEPLOY.md](QUICK_DEPLOY.md) - Начните отсюда
2. [GITHUB_AUTH_SETUP.md](GITHUB_AUTH_SETUP.md) - Авторизация
3. [DOCKER_README.md](DOCKER_README.md) - Команды
4. [deployment/DEPLOYMENT.md](deployment/DEPLOYMENT.md) - Детали
5. [deployment/DOCKER_VERSIONING.md](deployment/DOCKER_VERSIONING.md) - Версии
6. [configuration/ENV_CONFIGURATION.md](configuration/ENV_CONFIGURATION.md) - Настройка

---

**Начните прямо сейчас:** [Быстрый деплой →](QUICK_DEPLOY.md)
