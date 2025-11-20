# Структура scripts/

```
scripts/
│
├── 📘 README.md                    # Главная документация
├── 📁 STRUCTURE.md                 # Эта карта структуры
│
├── 🚀 deployment/                  # Скрипты развертывания
│   ├── deploy.sh                   # Деплой Linux/Mac ⭐ ГЛАВНЫЙ
│   └── deploy.ps1                  # Деплой Windows
│
├── 🌱 seed/                        # Тестовые данные
│   ├── main.go                     # Главный файл seeder
│   ├── clean_tasks.go              # Очистка задач
│   ├── setup-env.sh                # Настройка (Linux/Mac)
│   ├── setup-env.ps1               # Настройка (Windows)
│   ├── README.md                   # Документация seeder
│   ├── QUICKSTART.md               # Быстрый старт
│   ├── USAGE_RU.md                 # Использование (RU)
│   └── seeders/                    # Генераторы данных
│       ├── chats.go
│       ├── messages.go
│       ├── tasks.go
│       ├── users.go
│       ├── calendars.go
│       ├── polls.go
│       └── analytics.go
│
├── 🧪 testing/                     # Тестирование
│   └── test-integration.sh         # Интеграционные тесты
│
└── 🔧 utilities/                   # Утилиты
    ├── fix-passkey-settings.sh     # Исправление passkey настроек
    └── test_password.go            # Тестирование паролей
```

## Категории

### 🚀 deployment/ - Развертывание
**Назначение:** Автоматизация деплоя на production/staging

**Главные файлы:**
- `deploy.sh` - основной скрипт для Linux/Mac
- `deploy.ps1` - версия для Windows

**Использование:**
```bash
./scripts/deployment/deploy.sh deploy
```

**Документация:** [README.md](README.md#deployment)

---

### 🌱 seed/ - Тестовые данные
**Назначение:** Заполнение БД тестовыми данными для разработки

**Главные файлы:**
- `main.go` - точка входа
- `seeders/` - генераторы данных для каждой сущности

**Использование:**
```bash
cd scripts/seed
./setup-env.sh
go run main.go
```

**Документация:** [seed/README.md](seed/README.md)

---

### 🧪 testing/ - Тестирование
**Назначение:** Автоматизация тестирования

**Главные файлы:**
- `test-integration.sh` - интеграционные тесты

**Использование:**
```bash
./scripts/testing/test-integration.sh
```

---

### 🔧 utilities/ - Утилиты
**Назначение:** Вспомогательные инструменты

**Файлы:**
- `fix-passkey-settings.sh` - исправление настроек WebAuthn
- `test_password.go` - тестирование хеширования паролей

**Использование:**
```bash
./scripts/utilities/fix-passkey-settings.sh
cd scripts/utilities && go run test_password.go "password"
```

---

## Быстрая навигация

**Нужно задеплоить?** → [deployment/deploy.sh](deployment/deploy.sh)

**Нужны тестовые данные?** → [seed/README.md](seed/README.md)

**Запустить тесты?** → [testing/test-integration.sh](testing/test-integration.sh)

**Нужна утилита?** → [utilities/](utilities/)

---

## Использование по сценариям

### Сценарий 1: Первый деплой
```bash
# Используйте deployment/
./scripts/deployment/deploy.sh deploy
```

### Сценарий 2: Заполнить БД данными
```bash
# Используйте seed/
cd scripts/seed
./setup-env.sh
go run main.go
```

### Сценарий 3: Протестировать API
```bash
# Используйте testing/
./scripts/testing/test-integration.sh
```

### Сценарий 4: Исправить настройки
```bash
# Используйте utilities/
./scripts/utilities/fix-passkey-settings.sh
```

---

## Правила организации

При добавлении новых скриптов:

1. **Выберите категорию:**
   - `deployment/` - если связано с развертыванием
   - `seed/` - если заполняет данными
   - `testing/` - если тестирует
   - `utilities/` - для остального

2. **Создайте документацию** в комментариях скрипта

3. **Обновите [README.md](README.md)** с описанием

4. **Сделайте исполняемым:**
   ```bash
   chmod +x scripts/category/your-script.sh
   ```

---

**Главная документация:** [README.md](README.md)
