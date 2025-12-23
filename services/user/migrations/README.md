# Миграция для сброса зависших онлайн-статусов

## Проблема
После изменения логики обновления статусов пользователей некоторые пользователи "зависли" в статусе `online`, хотя давно вышли из приложения.

## Решение

### 1. Одноразовая миграция (для немедленного сброса)

Выполните SQL-миграцию вручную для сброса всех зависших статусов:

```bash
psql -h localhost -U your_user -d your_database -f services/user/migrations/reset_stuck_online_statuses.sql
```

Или выполните SQL напрямую в базе данных:

```sql
UPDATE users
SET
    status = 'offline',
    last_active_at = updated_at,
    updated_at = CURRENT_TIMESTAMP
WHERE status = 'online';
```

### 2. Автоматический сброс при старте chat-service

При каждом запуске `chat-service` будет автоматически вызываться endpoint `POST /internal/users/reset-online-statuses` в `user-service`, который сбросит все онлайн-статусы в offline.

Это гарантирует, что после перезапуска сервиса все пользователи начнут с чистого состояния.

### 3. Периодическая проверка (каждые 5 минут)

`chat-service` автоматически запускает периодическую проверку каждые 5 минут, которая:

1. Получает список всех подключенных к WebSocket пользователей
2. Отправляет запрос в `user-service` с этим списком
3. `user-service` сбрасывает статус всех пользователей, которые отмечены как `online`, но не находятся в списке подключенных

Endpoint: `POST /internal/users/cleanup-statuses`

Payload:
```json
{
  "connected_user_ids": [1, 2, 3, 4, 5]
}
```

## Архитектура решения

### Chat Service (WebSocket Hub)

**Файл:** `services/chat/websocket/hub.go`

#### При старте:
```go
func (h *Hub) Run() {
    // Сброс всех зависших статусов при запуске
    go h.resetStuckOnlineStatuses()

    // Запуск периодической проверки каждые 5 минут
    go h.periodicStatusCleanup()

    // ...
}
```

#### Функции:
- `resetStuckOnlineStatuses()` - вызывает `POST /internal/users/reset-online-statuses`
- `periodicStatusCleanup()` - запускает проверку каждые 5 минут
- `cleanupInactiveStatuses()` - отправляет список подключенных пользователей в `POST /internal/users/cleanup-statuses`

### User Service

**Файлы:**
- `services/user/handlers/user_handler.go` - HTTP handlers
- `services/user/usecase/user_usecase.go` - бизнес-логика
- `services/user/repository/user_repository.go` - работа с БД

#### Новые endpoints:

**POST /internal/users/reset-online-statuses**
- Сбрасывает все статусы `online` в `offline`
- Обновляет `last_active_at` на текущее время
- Возвращает количество обновленных пользователей

**POST /internal/users/cleanup-statuses**
- Принимает список ID подключенных пользователей
- Сбрасывает статус в `offline` для всех, кто в `online`, но не в списке
- Возвращает количество обновленных пользователей

#### Пример ответа:
```json
{
  "message": "Online statuses reset successfully",
  "users_reset": 15,
  "request_id": "abc-123-def"
}
```

## Логирование

### Chat Service логи:
```
🔄 Resetting stuck online statuses on startup...
✅ Successfully reset all stuck online statuses to offline

🔄 Started periodic status cleanup (every 5 minutes)
🔍 Running inactive status cleanup...
✅ Status cleanup completed (3 users currently connected)
```

### User Service логи:
```json
{
  "level": "info",
  "msg": "Successfully reset all online statuses to offline",
  "request_id": "abc-123",
  "count": 15
}

{
  "level": "info",
  "msg": "Successfully cleaned up disconnected user statuses",
  "request_id": "def-456",
  "connected_users": 3,
  "users_set_offline": 12
}
```

## Преимущества решения

1. **Автоматический сброс при старте** - гарантирует чистое состояние после перезапуска
2. **Периодическая проверка** - предотвращает накопление зависших статусов во время работы
3. **Не требует вмешательства** - работает полностью автоматически
4. **Логирование** - легко отслеживать работу механизмов очистки
5. **Безопасность** - не влияет на реально подключенных пользователей

## Тестирование

### Проверка работы автоматического сброса:

1. Вручную установите статусы пользователей в `online`:
```sql
UPDATE users SET status = 'online' WHERE id IN (1, 2, 3);
```

2. Перезапустите `chat-service`

3. Проверьте логи - должно появиться сообщение о сбросе статусов

4. Проверьте в БД - все пользователи должны быть `offline`:
```sql
SELECT id, email, status FROM users WHERE id IN (1, 2, 3);
```

### Проверка периодической очистки:

1. Подключите одного пользователя через WebSocket
2. Установите статусы других пользователей в `online`
3. Подождите 5 минут
4. Проверьте логи - должна пройти очистка
5. Проверьте БД - только подключенный пользователь должен быть `online`

## Отключение функционала (если нужно)

Если по какой-то причине нужно отключить автоматическую очистку, закомментируйте в `services/chat/websocket/hub.go`:

```go
func (h *Hub) Run() {
    log.Println("WebSocket hub started")

    // Закомментируйте эти строки:
    // go h.resetStuckOnlineStatuses()
    // go h.periodicStatusCleanup()

    go h.updateMetrics()
    // ...
}
```
