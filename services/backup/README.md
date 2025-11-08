# Backup Service

Микросервис для управления бэкапами базы данных PostgreSQL в системе Tachyon Messenger.

## Возможности

- ✅ Создание ручных бэкапов
- ✅ Автоматические бэкапы каждые 24 часа
- ✅ Восстановление базы данных из бэкапа
- ✅ Скачивание файлов бэкапов
- ✅ Удаление старых бэкапов
- ✅ Статистика по бэкапам
- ✅ Интеграция с Analytics Dashboard

## API Эндпоинты

Все эндпоинты требуют авторизации с ролью `super_admin`.

### Получить статистику

```bash
GET /api/v1/backups/stats
```

**Ответ:**
```json
{
  "total_backups": 5,
  "successful_backups": 4,
  "failed_backups": 1,
  "pending_backups": 0,
  "in_progress_backups": 0,
  "last_backup": {
    "id": 5,
    "file_name": "backup_20251108_120000.sql",
    "file_size": 1048576,
    "status": "completed",
    "type": "automatic",
    "created_at": "2025-11-08T12:00:00Z"
  },
  "total_size": 5242880
}
```

### Список бэкапов

```bash
GET /api/v1/backups?page=1&page_size=10
```

**Параметры:**
- `page` - номер страницы (по умолчанию: 1)
- `page_size` - количество элементов на странице (по умолчанию: 10)

**Ответ:**
```json
{
  "backups": [...],
  "total": 5,
  "page": 1,
  "page_size": 10,
  "total_pages": 1
}
```

### Создать бэкап

```bash
POST /api/v1/backups
Content-Type: application/json

{
  "type": "manual",
  "description": "Before system upgrade"
}
```

**Ответ:**
```json
{
  "id": 6,
  "file_name": "backup_20251108_130000.sql",
  "status": "pending",
  "type": "manual",
  "description": "Before system upgrade",
  "created_at": "2025-11-08T13:00:00Z"
}
```

### Восстановить из бэкапа

```bash
POST /api/v1/backups/:id/restore
```

**⚠️ ВНИМАНИЕ:** Эта операция заменит текущие данные в базе!

### Скачать бэкап

```bash
GET /api/v1/backups/:id/download
```

Скачивает файл бэкапа в формате SQL.

### Удалить бэкап

```bash
DELETE /api/v1/backups/:id
```

## Автоматические бэкапы

Сервис автоматически создает бэкапы по следующему расписанию:

- **Первый бэкап**: через 5 минут после запуска сервиса
- **Последующие**: каждые 24 часа

Все автоматические бэкапы помечаются типом `automatic`.

## Хранение бэкапов

Бэкапы хранятся в:
- **Docker volume**: `tachyon_backup_data`
- **Путь внутри контейнера**: `/app/backups`
- **Формат**: Plain SQL (`.sql` файлы)

## Технические детали

### База данных

Таблица `backups`:
```sql
CREATE TABLE backups (
  id SERIAL PRIMARY KEY,
  file_name VARCHAR(255) NOT NULL UNIQUE,
  file_path VARCHAR(512) NOT NULL,
  file_size BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  type VARCHAR(20) NOT NULL DEFAULT 'manual',
  created_by INTEGER NOT NULL,
  description VARCHAR(500),
  error_message TEXT,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
```

### Статусы бэкапов

- `pending` - в очереди на создание
- `in_progress` - создается в данный момент
- `completed` - успешно завершен
- `failed` - ошибка при создании

### Типы бэкапов

- `manual` - создан вручную пользователем
- `automatic` - создан автоматически по расписанию
- `scheduled` - создан по пользовательскому расписанию

## Интеграция с Frontend

### Страница управления бэкапами

Доступна по адресу `/backups` в dashboard.

**Функционал:**
- Просмотр всех бэкапов с фильтрацией
- Создание нового бэкапа
- Скачивание бэкапа
- Восстановление из бэкапа
- Удаление бэкапа
- Статистика в реальном времени

### Analytics Dashboard

Статистика бэкапов автоматически отображается в разделе Analytics:
- Общее количество бэкапов
- Успешные/неудачные бэкапы
- Статус последнего бэкапа
- Общий размер всех бэкапов

## Безопасность

- Доступ только для пользователей с ролью `super_admin`
- Все операции логируются
- Восстановление требует дополнительного подтверждения
- Файлы бэкапов хранятся в защищенном volume

## Мониторинг

### Логи

```bash
# Просмотр логов сервиса
docker logs tachyon-backup-service

# Отслеживание логов в реальном времени
docker logs -f tachyon-backup-service
```

### Health Check

```bash
curl http://localhost:8089/health
```

## Восстановление после сбоя

Если бэкап-сервис недоступен:

1. Проверьте статус контейнера:
```bash
docker ps -a | grep backup
```

2. Перезапустите сервис:
```bash
docker-compose restart backup-service
```

3. Проверьте логи:
```bash
docker logs tachyon-backup-service --tail 100
```

## Рекомендации

1. **Регулярно проверяйте бэкапы**: Периодически тестируйте восстановление
2. **Мониторьте место на диске**: Следите за размером volume
3. **Храните копии вне сервера**: Скачивайте критически важные бэкапы
4. **Удаляйте старые бэкапы**: Не храните неограниченное количество

## Troubleshooting

### Бэкап завис в статусе "in_progress"

```bash
# Проверьте процессы pg_dump
docker exec tachyon-backup-service ps aux | grep pg_dump

# Перезапустите сервис
docker-compose restart backup-service
```

### Недостаточно места для бэкапа

```bash
# Проверьте использование диска
docker exec tachyon-backup-service df -h /app/backups

# Удалите старые бэкапы через API или вручную
```

### Ошибка при восстановлении

Проверьте:
1. Целостность файла бэкапа
2. Права доступа к базе данных
3. Логи PostgreSQL

## Переменные окружения

```env
BACKUP_DIR=/app/backups              # Директория для хранения бэкапов
SERVER_PORT=8089                      # Порт сервиса
DATABASE_URL=postgres://...          # URL базы данных
```
