# Управление версиями приложений

Система управления версиями приложений позволяет загружать, хранить и распространять приложения для Windows, Android и iOS через централизованную админ-панель.

## 📋 Оглавление
- [Возможности](#возможности)
- [Настройка](#настройка)
- [Загрузка версий](#загрузка-версий)
- [API Endpoints](#api-endpoints)
- [Email-приглашения](#email-приглашения)
- [Обновления приложений](#обновления-приложений)

## 🎯 Возможности

- ✅ **Windows**: Загрузка .exe или .msix файлов, прямое скачивание
- ✅ **Android**: Загрузка .apk файлов, прямое скачивание
- ✅ **iOS**: Ссылка на App Store (файлы не хранятся)
- ✅ **Версионирование**: Семантическое версионирование (X.Y.Z)
- ✅ **Активация**: Только одна активная версия на платформу
- ✅ **Статистика**: Счетчик скачиваний, размеры файлов
- ✅ **Критические обновления**: Флаг для принудительных обновлений
- ✅ **Безопасность**: SHA-256 checksum для проверки целостности
- ✅ **Changelog**: История изменений для каждой версии

## ⚙️ Настройка

### 1. Переменные окружения

Добавьте в `.env`:

```bash
# Путь для хранения файлов приложений (Windows/Android)
APP_STORAGE_PATH=./storage/app-versions

# Windows App URL (опционально, если пусто - используется /downloads/windows/latest)
WINDOWS_APP_URL=

# Backend URL (используется для генерации ссылок)
BACKEND_URL=http://localhost:8080
```

### 2. Создание директории хранилища

Система автоматически создаст директорию при первом запуске, но можно создать вручную:

```bash
mkdir -p storage/app-versions/windows
mkdir -p storage/app-versions/android
```

### 3. Миграция базы данных

Миграция выполняется автоматически при запуске user-service. Таблица `app_versions` будет создана.

## 📦 Загрузка версий

### Через админ-панель (Dashboard)

1. Откройте **Администрирование → Версии приложений**
2. Нажмите **"Загрузить новую версию"**
3. Заполните форму:
   - **Платформа**: Windows, Android или iOS
   - **Версия**: Формат X.Y.Z (например, 1.2.3)
   - **Файл**: .exe/.msix для Windows, .apk для Android
   - **Changelog**: Описание изменений (опционально)
   - **Критическое обновление**: Чекбокс для принудительных обновлений
   - **App Store URL**: Для iOS вместо файла

4. Нажмите **"Загрузить"**

### Через API

#### Загрузка Windows/Android версии

```bash
curl -X POST http://localhost:8080/api/v1/admin/app-versions \
  -H "X-Session-ID: your-session-id" \
  -F "platform=windows" \
  -F "version=1.0.0" \
  -F "changelog=Первый релиз" \
  -F "is_critical=false" \
  -F "file=@tachyon-1.0.0.exe"
```

#### Загрузка iOS версии

```bash
curl -X POST http://localhost:8080/api/v1/admin/app-versions \
  -H "X-Session-ID: your-session-id" \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "ios",
    "version": "1.0.0",
    "changelog": "Первый релиз в App Store",
    "store_url": "https://apps.apple.com/app/tachyon/id123456789"
  }'
```

## 🔌 API Endpoints

### Admin Endpoints (требуется авторизация)

| Метод | Endpoint | Описание |
|-------|----------|----------|
| POST | `/api/v1/admin/app-versions` | Загрузить новую версию |
| GET | `/api/v1/admin/app-versions` | Список всех версий |
| GET | `/api/v1/admin/app-versions/stats` | Статистика |
| GET | `/api/v1/admin/app-versions/:id` | Получить версию по ID |
| PUT | `/api/v1/admin/app-versions/:id` | Обновить метаданные |
| DELETE | `/api/v1/admin/app-versions/:id` | Удалить версию |
| POST | `/api/v1/admin/app-versions/:id/activate` | Активировать версию |

### Public Endpoints (без авторизации)

| Метод | Endpoint | Описание |
|-------|----------|----------|
| GET | `/api/v1/app-versions/latest` | Последние версии всех платформ |
| GET | `/api/v1/app-versions/latest/{platform}` | Последняя версия платформы |
| GET | `/downloads/{platform}/latest` | Скачать последнюю версию |
| GET | `/downloads/{platform}/{version}` | Скачать конкретную версию |

### Примеры запросов

#### Получить последние версии

```bash
curl http://localhost:8080/api/v1/app-versions/latest
```

Ответ:
```json
{
  "windows": {
    "id": 1,
    "platform": "windows",
    "version": "1.2.3",
    "changelog": "Исправлены баги...",
    "is_critical": false,
    "is_active": true,
    "download_count": 42,
    "file_size": 52428800,
    "release_date": "2025-12-19T10:00:00Z"
  },
  "android": {...},
  "ios": {...}
}
```

#### Скачать последнюю версию Windows

```bash
curl -O http://localhost:8080/downloads/windows/latest
```

## 📧 Email-приглашения

Шаблон email-приглашения автоматически включает кнопки для скачивания приложений:

- **🪟 Windows** - Прямая ссылка на скачивание последней версии
- **🤖 Android** - Ссылка на Google Play или прямое скачивание
- **🍎 iOS** - Ссылка на App Store

### Настройка ссылок

В `.env` можно переопределить URL:

```bash
WINDOWS_APP_URL=https://github.com/yourorg/tachyon/releases/latest/download/tachyon.exe
GOOGLE_PLAY_URL=https://play.google.com/store/apps/details?id=com.tachyon.messenger
APP_STORE_URL=https://apps.apple.com/app/tachyon-messenger/id123456789
```

## 🔄 Обновления приложений

### Проверка обновлений из приложения

Приложения могут проверять наличие новых версий:

```javascript
// Пример для Windows/Android приложения
const response = await fetch('http://your-backend/api/v1/app-versions/latest/windows');
const latestVersion = await response.json();

if (latestVersion.version > currentVersion) {
  if (latestVersion.is_critical) {
    // Принудительное обновление
    forceUpdate(latestVersion);
  } else {
    // Предложить обновление
    offerUpdate(latestVersion);
  }
}
```

### Структура ответа

```json
{
  "id": 5,
  "platform": "windows",
  "version": "1.3.0",
  "changelog": "- Новая функция X\n- Исправлен баг Y",
  "is_critical": false,
  "is_active": true,
  "download_count": 156,
  "file_size": 54525952,
  "checksum": "sha256:a1b2c3d4...",
  "release_date": "2025-12-20T12:00:00Z"
}
```

### Критические обновления

Если версия помечена как `is_critical: true`, приложение должно:
1. Принудительно загрузить обновление
2. Не давать продолжить работу без обновления
3. Показать сообщение о критичности обновления

## 💾 Хранение файлов

### Структура директорий

```
storage/app-versions/
├── windows/
│   ├── windows-1.0.0.exe
│   ├── windows-1.1.0.exe
│   └── windows-1.2.0.exe
└── android/
    ├── android-1.0.0.apk
    └── android-1.1.0.apk
```

### База данных

Таблица `app_versions`:

| Поле | Тип | Описание |
|------|-----|----------|
| id | INTEGER | Первичный ключ |
| platform | VARCHAR(20) | windows, android, ios |
| version | VARCHAR(50) | Версия (X.Y.Z) |
| changelog | TEXT | История изменений |
| is_critical | BOOLEAN | Критическое обновление |
| is_active | BOOLEAN | Активная версия |
| download_count | BIGINT | Счетчик скачиваний |
| file_path | VARCHAR(500) | Путь к файлу |
| file_size | BIGINT | Размер в байтах |
| checksum | VARCHAR(64) | SHA-256 |
| store_url | VARCHAR(500) | URL в магазине (iOS) |
| release_date | TIMESTAMP | Дата релиза |
| uploaded_by_id | INTEGER | ID админа |

## 🔒 Безопасность

### Checksum проверка

При загрузке файла автоматически вычисляется SHA-256 checksum. Приложения могут проверить целостность файла:

```javascript
const downloadedChecksum = await calculateSHA256(downloadedFile);
if (downloadedChecksum !== expectedChecksum) {
  throw new Error('File integrity check failed');
}
```

### Права доступа

- **Загрузка версий**: Только админы и супер-админы
- **Удаление версий**: Только супер-админы
- **Просмотр и скачивание**: Публичный доступ

## 📊 Статистика

Админ-панель показывает:
- Общее количество версий
- Версии по платформам
- Общее количество скачиваний
- Скачивания по платформам
- Занятое место на диске

## 🚀 Production рекомендации

### 1. Используйте CDN

Для больших файлов рекомендуется использовать CDN:

```bash
WINDOWS_APP_URL=https://cdn.yourdomain.com/apps/tachyon-windows-latest.exe
```

### 2. Docker Volume

Для production создайте постоянный volume:

```yaml
volumes:
  app-storage:
    driver: local

services:
  user-service:
    volumes:
      - app-storage:/app/storage/app-versions
```

### 3. Backup

Регулярно делайте бэкапы:

```bash
# Бэкап файлов
tar -czf app-versions-backup-$(date +%Y%m%d).tar.gz storage/app-versions/

# Бэкап БД (app_versions таблица)
pg_dump -t app_versions tachyon_messenger > app-versions-db-backup.sql
```

### 4. Мониторинг

Следите за:
- Размером директории `storage/app-versions`
- Количеством скачиваний
- Ошибками при скачивании

## 🐛 Troubleshooting

### Ошибка "file not found on disk"

Проверьте:
1. Существует ли директория `APP_STORAGE_PATH`
2. Есть ли права на запись/чтение
3. Правильно ли указан путь в `.env`

### Ошибка "invalid file extension"

Допустимые расширения:
- Windows: `.exe`, `.msix`
- Android: `.apk`

### Большой размер файла

Увеличьте лимит в nginx/gateway:

```nginx
client_max_body_size 500M;
```

И в user-service (`main.go`):
```go
router.MaxMultipartMemory = 500 << 20  // 500 MB
```

## 📝 Changelog формат

Рекомендуется использовать Markdown:

```markdown
## Что нового
- Добавлена функция X
- Улучшена производительность Y

## Исправления
- Исправлен баг с Z
- Устранена проблема с W
```

## 🔗 Полезные ссылки

- [Semantic Versioning](https://semver.org/)
- [App Store Connect](https://appstoreconnect.apple.com/)
- [Google Play Console](https://play.google.com/console)
