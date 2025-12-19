# Quick Start: Управление версиями приложений

Быстрая инструкция по настройке и использованию системы управления версиями приложений.

## ⚡ Быстрый старт (5 минут)

### 1. Настройка окружения

Добавьте в `.env` (опционально, работает с дефолтными значениями):

```bash
# Путь для хранения файлов (по умолчанию: ./storage/app-versions)
APP_STORAGE_PATH=./storage/app-versions

# Backend URL для генерации ссылок
BACKEND_URL=http://localhost:8080
```

### 2. Запустите сервисы

```bash
# Backend автоматически создаст таблицу и директории
docker-compose up -d

# Или локально
go run services/user/main.go
```

### 3. Откройте админ-панель

1. Перейдите в Dashboard: `http://localhost:5173`
2. Войдите как админ
3. Откройте: **Администрирование → Версии приложений**

### 4. Загрузите первую версию

**Для Windows:**
1. Нажмите **"+ Загрузить новую версию"**
2. Выберите платформу: **Windows**
3. Введите версию: `1.0.0`
4. Выберите файл: `tachyon.exe` или `tachyon.msix`
5. Добавьте changelog (опционально)
6. Нажмите **"Загрузить"**

**Для Android:**
1. Выберите платформу: **Android**
2. Версия: `1.0.0`
3. Файл: `tachyon.apk`
4. Загрузите

**Для iOS:**
1. Выберите платформу: **iOS**
2. Версия: `1.0.0`
3. **Вместо файла** укажите App Store URL
4. Загрузите

## 📥 Использование

### Скачивание последней версии

Пользователи могут скачать приложение по ссылкам:

- **Windows**: `http://your-backend/downloads/windows/latest`
- **Android**: `http://your-backend/downloads/android/latest`
- **iOS**: Используется ссылка на App Store из настроек

### Email-приглашения

При отправке приглашений автоматически добавляются кнопки:
- 🪟 Windows
- 🤖 Android
- 🍎 iOS

Пользователи могут выбрать свою платформу и скачать приложение.

### API для проверки обновлений

Приложения могут проверять обновления:

```bash
# Получить последнюю версию
curl http://your-backend/api/v1/app-versions/latest/windows

# Ответ
{
  "version": "1.0.0",
  "is_critical": false,
  "changelog": "...",
  "file_size": 52428800
}
```

## 🔧 Управление версиями

### Активация другой версии

1. В списке версий найдите нужную
2. Нажмите **"Активировать"**
3. Эта версия станет доступна по `/downloads/{platform}/latest`

### Критическое обновление

1. При загрузке версии установите чекбокс **"Критическое обновление"**
2. Или обновите существующую версию
3. Приложения должны проверять флаг `is_critical` и принудительно обновляться

### Удаление версии

1. Найдите версию в списке
2. Нажмите **"Удалить"**
3. Файл будет удален с диска и из БД

## 📊 Статистика

В админ-панели доступна статистика:
- Количество версий по платформам
- Счетчики скачиваний
- Общий размер хранилища

## 🚨 Важно знать

### Формат версий

Используйте семантическое версионирование: `X.Y.Z`
- ✅ Правильно: `1.0.0`, `1.2.3`, `2.0.0`
- ❌ Неправильно: `1.0`, `v1.0.0`, `1.0.0-beta`

### Расширения файлов

- Windows: только `.exe` или `.msix`
- Android: только `.apk`
- iOS: без файла, только URL

### Размер файлов

По умолчанию максимум **32 MB**. Для больших файлов:

В `services/user/main.go`:
```go
router.MaxMultipartMemory = 500 << 20  // 500 MB
```

В nginx/gateway:
```nginx
client_max_body_size 500M;
```

### Хранение файлов

Файлы хранятся в:
```
storage/app-versions/
├── windows/windows-1.0.0.exe
└── android/android-1.0.0.apk
```

iOS файлы не хранятся (только ссылка на App Store).

## 🔐 Безопасность

- Загрузка версий: **только админы**
- Удаление версий: **только супер-админы**
- Скачивание: **публичный доступ** (без авторизации)
- Каждый файл имеет SHA-256 checksum для проверки целостности

## 📱 Интеграция в приложения

### Проверка обновлений (псевдокод)

```javascript
async function checkForUpdates() {
  const response = await fetch('/api/v1/app-versions/latest/windows');
  const latest = await response.json();

  if (latest.version > CURRENT_VERSION) {
    if (latest.is_critical) {
      // Принудительное обновление
      await forceDownloadAndInstall(latest);
    } else {
      // Предложить обновление
      showUpdateDialog(latest);
    }
  }
}
```

### Скачивание и проверка целостности

```javascript
async function downloadAndVerify(url, expectedChecksum) {
  const file = await downloadFile(url);
  const checksum = await calculateSHA256(file);

  if (checksum !== expectedChecksum) {
    throw new Error('File corrupted!');
  }

  return file;
}
```

## 🆘 Помощь

### Логи

Проверьте логи user-service:
```bash
docker logs user-service
```

### Проверка API

```bash
# Проверить доступность
curl http://localhost:8080/api/v1/app-versions/latest

# Скачать файл
curl -O http://localhost:8080/downloads/windows/latest
```

### Частые проблемы

1. **"Файл не найден"** → Проверьте `APP_STORAGE_PATH`
2. **"Неверное расширение"** → Используйте .exe/.msix/.apk
3. **"Слишком большой файл"** → Увеличьте лимиты

## 📖 Дополнительная документация

Полная документация: [APP_VERSIONS_MANAGEMENT.md](./APP_VERSIONS_MANAGEMENT.md)
