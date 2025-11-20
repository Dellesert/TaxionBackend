# Структура документации

```
docs/
│
├── README.md                          # Главная страница документации
│
├── 🚀 QUICK_DEPLOY.md                # Быстрый деплой за 20 минут (START HERE)
├── 🔑 GITHUB_AUTH_SETUP.md           # Настройка авторизации в GHCR
├── 📋 DOCKER_README.md                # Краткая справка по командам
│
├── deployment/                        # Развертывание
│   ├── DEPLOYMENT.md                  # Полное руководство по деплою
│   └── DOCKER_VERSIONING.md           # Управление версиями образов
│
├── configuration/                     # Конфигурация
│   ├── ENV_CONFIGURATION.md           # Настройка переменных окружения
│   └── ENVIRONMENT_COMPARISON.md      # Development vs Production
│
├── guides/                            # Руководства
│   └── FCM_PUSH_NOTIFICATIONS.md      # Настройка Firebase push-уведомлений
│
├── api/                               # API документация
│   └── API_ERROR_RESPONSES.md         # Справочник по ошибкам API
│
└── archived/                          # Архив старых документов
    ├── PRODUCTION_DEPLOYMENT.md
    ├── PRODUCTION_QUICKSTART.md
    ├── DEPLOYMENT_CHECKLIST.md
    └── DEPLOYMENT_REQUIREMENTS.md
```

## Категории документов

### 🟢 Для начинающих
- QUICK_DEPLOY.md
- GITHUB_AUTH_SETUP.md
- DOCKER_README.md

### 🟡 Для продвинутых
- deployment/DEPLOYMENT.md
- deployment/DOCKER_VERSIONING.md
- guides/FCM_PUSH_NOTIFICATIONS.md

### 🔴 Справочная информация
- configuration/ENV_CONFIGURATION.md
- configuration/ENVIRONMENT_COMPARISON.md
- api/API_ERROR_RESPONSES.md

## Последовательность изучения

```
1. QUICK_DEPLOY.md
   ↓
2. GITHUB_AUTH_SETUP.md
   ↓
3. DOCKER_README.md
   ↓
4. deployment/DEPLOYMENT.md
   ↓
5. deployment/DOCKER_VERSIONING.md
   ↓
6. configuration/ENV_CONFIGURATION.md
```

## Быстрый поиск

- **Первый деплой?** → QUICK_DEPLOY.md
- **Проблемы с авторизацией?** → GITHUB_AUTH_SETUP.md
- **Забыли команду?** → DOCKER_README.md
- **Нужны детали?** → deployment/DEPLOYMENT.md
- **Обновление версии?** → deployment/DOCKER_VERSIONING.md
- **Настройка push?** → guides/FCM_PUSH_NOTIFICATIONS.md
- **Ошибка API?** → api/API_ERROR_RESPONSES.md
