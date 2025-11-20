# Scripts - Скрипты автоматизации

Коллекция скриптов для автоматизации развертывания, тестирования и обслуживания Tachyon Messenger.

## 📁 Структура

```
scripts/
├── README.md                   # Этот файл
│
├── deployment/                 # Скрипты развертывания
│   ├── deploy.sh              # Деплой для Linux/Mac ⭐
│   └── deploy.ps1             # Деплой для Windows
│
├── seed/                       # Скрипты заполнения тестовыми данными
│   ├── main.go                # Главный файл seeder
│   ├── setup-env.sh           # Настройка окружения (Linux/Mac)
│   ├── setup-env.ps1          # Настройка окружения (Windows)
│   ├── README.md              # Документация seeder
│   ├── QUICKSTART.md          # Быстрый старт
│   ├── USAGE_RU.md            # Использование на русском
│   └── seeders/               # Генераторы данных
│
├── testing/                    # Скрипты тестирования
│   └── test-integration.sh    # Интеграционные тесты
│
└── utilities/                  # Утилиты
    ├── fix-passkey-settings.sh  # Исправление настроек passkey
    └── test_password.go         # Тестирование паролей
```

---

## 🚀 Deployment - Развертывание

### deploy.sh / deploy.ps1

Основной скрипт для развертывания на production.

**Linux/Mac:**
```bash
./scripts/deployment/deploy.sh deploy
```

**Windows:**
```powershell
.\scripts\deployment\deploy.ps1 deploy
```

**Команды:**
```bash
deploy      # Полный деплой (pull + up + health check)
pull        # Pull образов из GHCR
status      # Статус всех сервисов
health      # Health check всех сервисов
logs        # Просмотр логов [service]
down        # Остановка всех сервисов
restart     # Перезапуск [service]
```

**Примеры:**
```bash
# Полный деплой
./scripts/deployment/deploy.sh deploy

# Логи конкретного сервиса
./scripts/deployment/deploy.sh logs chat-service

# Перезапуск сервиса
./scripts/deployment/deploy.sh restart user-service
```

**Документация:** [docs/QUICK_DEPLOY.md](../docs/QUICK_DEPLOY.md)

---

## 🌱 Seed - Тестовые данные

### Описание

Скрипты для заполнения базы данных тестовыми данными для разработки и демонстрации.

### Быстрый старт

**Linux/Mac:**
```bash
cd scripts/seed
./setup-env.sh              # Настройка окружения
go run main.go              # Запуск seeder
```

**Windows:**
```powershell
cd scripts\seed
.\setup-env.ps1             # Настройка окружения
go run main.go              # Запуск seeder
```

### Что создаётся

- 👥 **Пользователи** - демо аккаунты
- 💬 **Чаты** - групповые и личные чаты
- 📝 **Сообщения** - история переписки
- 📋 **Задачи** - примеры задач и проектов
- 📅 **События** - календарные события
- 📊 **Опросы** - примеры голосований

### Документация

- **[seed/README.md](seed/README.md)** - Полная документация
- **[seed/QUICKSTART.md](seed/QUICKSTART.md)** - Быстрый старт
- **[seed/USAGE_RU.md](seed/USAGE_RU.md)** - Использование на русском

---

## 🧪 Testing - Тестирование

### test-integration.sh

Запуск интеграционных тестов для всех сервисов.

**Использование:**
```bash
./scripts/testing/test-integration.sh
```

**Что проверяет:**
- Доступность всех эндпоинтов
- Корректность ответов API
- Интеграцию между сервисами
- Health checks

**Требования:**
- Запущенные сервисы
- curl установлен
- jq установлен (для парсинга JSON)

---

## 🔧 Utilities - Утилиты

### fix-passkey-settings.sh

Исправление настроек WebAuthn/Passkey в базе данных.

**Использование:**
```bash
./scripts/utilities/fix-passkey-settings.sh
```

**Что делает:**
- Проверяет настройки WebAuthn в БД
- Исправляет некорректные значения
- Добавляет отсутствующие параметры

---

### test_password.go

Утилита для тестирования хеширования паролей.

**Использование:**
```bash
cd scripts/utilities
go run test_password.go "your_password"
```

**Что делает:**
- Хеширует пароль bcrypt
- Проверяет корректность хеша
- Выводит результат

---

## 📋 Быстрая справка

### Деплой на production

```bash
# 1. Настройка авторизации
cat > ~/.github-credentials << 'EOF'
GITHUB_TOKEN=your_token
GITHUB_USERNAME=your_username
EOF
chmod 600 ~/.github-credentials

# 2. Деплой
./scripts/deployment/deploy.sh deploy

# 3. Проверка
./scripts/deployment/deploy.sh status
```

### Заполнение тестовыми данными

```bash
# 1. Настройка
cd scripts/seed
./setup-env.sh

# 2. Запуск
go run main.go

# 3. Проверка
# Откройте приложение и увидите тестовые данные
```

### Тестирование

```bash
# Интеграционные тесты
./scripts/testing/test-integration.sh
```

---

## 🛠️ Системные требования

### Deployment
- **Linux/Mac:** bash, docker, docker-compose
- **Windows:** PowerShell, docker, docker-compose

### Seed
- **Go 1.21+**
- **PostgreSQL** (доступ к БД)
- **Переменные окружения** (.env файл)

### Testing
- **curl**
- **jq** (для парсинга JSON)
- **Запущенные сервисы**

---

## 📚 Дополнительная документация

- **Deployment:** [docs/QUICK_DEPLOY.md](../docs/QUICK_DEPLOY.md)
- **Авторизация:** [docs/GITHUB_AUTH_SETUP.md](../docs/GITHUB_AUTH_SETUP.md)
- **Seeder:** [seed/README.md](seed/README.md)
- **Конфигурация:** [docs/configuration/ENV_CONFIGURATION.md](../docs/configuration/ENV_CONFIGURATION.md)

---

## 🆘 Troubleshooting

### Скрипт deploy.sh не запускается

**Проблема:** Permission denied

**Решение:**
```bash
chmod +x scripts/deployment/deploy.sh
./scripts/deployment/deploy.sh deploy
```

---

### Seeder не может подключиться к БД

**Проблема:** Connection refused

**Решение:**
1. Проверьте что PostgreSQL запущен:
   ```bash
   docker compose ps postgres
   ```
2. Проверьте .env файл в scripts/seed/
3. Запустите setup-env.sh заново

---

### Интеграционные тесты падают

**Проблема:** Сервисы не отвечают

**Решение:**
1. Проверьте что все сервисы запущены:
   ```bash
   docker compose ps
   ```
2. Подождите 1-2 минуты после старта
3. Проверьте health:
   ```bash
   ./scripts/deployment/deploy.sh health
   ```

---

## 🤝 Поддержка

Возникли проблемы со скриптами?

1. Проверьте документацию в соответствующей папке
2. Посмотрите логи: `./scripts/deployment/deploy.sh logs`
3. Создайте Issue в репозитории

---

## 📝 Разработка новых скриптов

При добавлении новых скриптов:

1. **Выберите категорию:**
   - `deployment/` - для развертывания
   - `seed/` - для тестовых данных
   - `testing/` - для тестов
   - `utilities/` - для утилит

2. **Добавьте документацию:**
   - Опишите назначение
   - Укажите использование
   - Добавьте примеры

3. **Обновите этот README:**
   - Добавьте в структуру
   - Опишите команды
   - Добавьте в быструю справку

---

**Начните с:** [deployment/deploy.sh](deployment/deploy.sh) для деплоя или [seed/README.md](seed/README.md) для тестовых данных
