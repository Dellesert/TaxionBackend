# Быстрый старт

## 1. Установите зависимости

```bash
go get github.com/brianvoe/gofakeit/v7
go get github.com/joho/godotenv
```

## 2. Настройте окружение для работы с Docker

### Вариант А: Использовать скрипт-помощник (рекомендуется) 🚀

**На Windows (PowerShell):**
```powershell
# 1. Настройте .env для локального запуска
.\scripts\seed\setup-env.ps1 local

# 2. Запустите Docker контейнеры
docker compose up -d postgres redis

# 3. Запустите seeding (переходите к шагу 3)
```

**На Linux/Mac (Bash):**
```bash
# 1. Сделайте скрипт исполняемым (один раз)
chmod +x scripts/seed/setup-env.sh

# 2. Настройте .env для локального запуска
./scripts/seed/setup-env.sh local

# 3. Запустите Docker контейнеры
docker compose up -d postgres redis

# 4. Дождитесь запуска (5-10 секунд)
sleep 5

# 5. Запустите seeding (переходите к шагу 3)
```

> **После завершения работы** верните настройки:
> - Windows: `.\scripts\seed\setup-env.ps1 docker`
> - Linux/Mac: `./scripts/seed/setup-env.sh docker`

### Вариант Б: Изменить .env вручную

1. **Запустите PostgreSQL и Redis в Docker:**
   ```bash
   docker compose up -d postgres redis
   ```

2. **Временно измените `.env` для локального запуска скриптов:**

   Откройте файл `.env` в корне проекта и измените следующие строки:

   ```env
   # Было (для Docker):
   DATABASE_URL=postgres://tachyon_user:tachyon_password@postgres:5432/tachyon_messenger?sslmode=disable
   REDIS_URL=redis://:redis_password@redis:6379

   # Стало (для локального запуска):
   DATABASE_URL=postgres://tachyon_user:tachyon_password@localhost:5432/tachyon_messenger?sslmode=disable
   REDIS_URL=redis://:redis_password@localhost:6379
   ```

   > **Важно!** После работы со скриптами верните обратно `postgres` и `redis` вместо `localhost`.

### Если база данных запущена локально:

Убедитесь, что в `.env` файле указан `localhost`:

```env
DATABASE_URL=postgres://tachyon_user:tachyon_password@localhost:5432/tachyon_messenger?sslmode=disable
```

## 3. Запустите генератор

### Вариант 1: Все данные (рекомендуется для начала)

```bash
go run scripts/seed/main.go --all --clean
```

Это создаст:
- 50 пользователей в 8 отделах
- 30 чатов с сообщениями
- 100 задач разных статусов
- 20 опросов
- 80 календарных событий

### Вариант 2: Свое количество данных

```bash
go run scripts/seed/main.go --all --clean \
  --user-count 30 \
  --chat-count 20 \
  --task-count 50 \
  --poll-count 10 \
  --event-count 40
```

## 4. Войдите в систему

После генерации данных вы можете войти как:

**Суперадмин:**
- Email: `admin@taxion.ru`
- Пароль: `password123`

**Любой другой пользователь:**
- Пароль для всех: `password123`

## 5. Верните настройки Docker (если нужно)

Если вы изменяли `.env` для локального запуска скриптов и планируете запускать приложение в Docker, верните обратно настройки:

```env
# Верните для Docker:
DATABASE_URL=postgres://tachyon_user:tachyon_password@postgres:5432/tachyon_messenger?sslmode=disable
REDIS_URL=redis://:redis_password@redis:6379
```

## Готово!

Теперь ваша база данных заполнена тестовыми данными с:
- ✅ Русскими именами пользователей (Александр Иванов, Мария Петрова и т.д.)
- ✅ Реальными фотографиями из UI Faces API
- ✅ Цветными аватарками для групповых чатов
- ✅ Реалистичными сообщениями, задачами, опросами и событиями

Подробнее см. [README.md](README.md)
