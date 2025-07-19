# Admin API Testing Examples

## Предварительные требования

1. Запустить сервис и получить admin токен:

```bash
# Зарегистрировать админа
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "name": "Admin User",
    "password": "admin123",
    "role": "admin"
  }'

# Войти как админ
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "admin123"
  }' | jq

# Сохранить токен
export ADMIN_TOKEN="ваш_access_token_здесь"
```

## Администрирование пользователей

### 1. Получить список всех пользователей (админ)

```bash
curl -X GET "http://localhost:8081/admin/users?limit=10&offset=0" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

**Ожидаемый ответ (200 OK):**
```json
{
  "users": [
    {
      "id": 1,
      "email": "admin@example.com",
      "name": "Admin User",
      "role": "admin",
      "status": "online",
      "is_active": true,
      "created_at": "2025-01-20T10:30:00Z"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0,
  "request_id": "req-123"
}
```

### 2. Создать пользователя (админ)

```bash
curl -X POST http://localhost:8081/admin/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "name": "New User",
    "password": "password123",
    "role": "employee",
    "department_id": 1,
    "position": "Developer"
  }' | jq
```

**Ожидаемый ответ (201 Created):**
```json
{
  "message": "User created successfully",
  "user": {
    "id": 2,
    "email": "newuser@example.com",
    "name": "New User",
    "role": "employee",
    "status": "offline",
    "department_id": 1,
    "position": "Developer",
    "is_active": true,
    "created_at": "2025-01-20T10:35:00Z"
  },
  "request_id": "req-124"
}
```

### 3. Обновить пользователя (админ)

```bash
curl -X PUT http://localhost:8081/admin/users/2 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated User Name",
    "position": "Senior Developer",
    "status": "online"
  }' | jq
```

### 4. Получить статистику пользователей

```bash
curl -X GET http://localhost:8081/admin/users/stats \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

**Ожидаемый ответ:**
```json
{
  "stats": {
    "total_users": 2,
    "active_users": 2,
    "inactive_users": 0,
    "online_users": 1
  },
  "request_id": "req-125"
}
```

### 5. Обновить роль пользователя

```bash
curl -X PUT http://localhost:8081/admin/users/2/role \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "manager"
  }' | jq
```

### 6. Обновить статус пользователя

```bash
curl -X PUT http://localhost:8081/admin/users/2/status \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "busy"
  }' | jq
```

### 7. Деактивировать пользователя

```bash
curl -X PUT http://localhost:8081/admin/users/2/deactivate \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

### 8. Активировать пользователя

```bash
curl -X PUT http://localhost:8081/admin/users/2/activate \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

## Администрирование отделов

### 1. Получить список отделов (админ)

```bash
curl -X GET http://localhost:8081/admin/departments \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

### 2. Создать отдел

```bash
curl -X POST http://localhost:8081/admin/departments \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "DevOps"
  }' | jq
```

### 3. Получить отдел с пользователями

```bash
curl -X GET http://localhost:8081/admin/departments/1/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

### 4. Обновить отдел

```bash
curl -X PUT http://localhost:8081/admin/departments/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Engineering Team"
  }' | jq
```

### 5. Удалить отдел

```bash
curl -X DELETE http://localhost:8081/admin/departments/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

## Системное администрирование (super_admin только)

### 1. Создать super_admin пользователя

```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@example.com",
    "name": "Super Admin",
    "password": "superadmin123",
    "role": "super_admin"
  }'

# Войти как super admin
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@example.com",
    "password": "superadmin123"
  }' | jq

export SUPER_ADMIN_TOKEN="ваш_super_admin_token"
```

### 2. Системный health check

```bash
curl -X GET http://localhost:8081/admin/system/health \
  -H "Authorization: Bearer $SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

### 3. Системная статистика

```bash
curl -X GET http://localhost:8081/admin/system/stats \
  -H "Authorization: Bearer $SUPER_ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

## Тестирование ошибок доступа

### 1. Доступ без токена

```bash
curl -X GET http://localhost:8081/admin/users \
  -H "Content-Type: application/json" | jq
```

**Ожидаемый ответ (401 Unauthorized):**
```json
{
  "error": "Authorization header is required"
}
```

### 2. Доступ с обычным пользователем

```bash
# Создать обычного пользователя
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "name": "Regular User",
    "password": "user123",
    "role": "employee"
  }'

# Войти как обычный пользователь
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "user123"
  }' | jq

export USER_TOKEN="user_access_token"

# Попытаться получить доступ к админ эндпоинту
curl -X GET http://localhost:8081/admin/users \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" | jq
```

**Ожидаемый ответ (403 Forbidden):**
```json
{
  "error": "Admin access required",
  "message": "This action requires administrator privileges",
  "request_id": "req-126"
}
```

### 3. Доступ админа к super admin эндпоинту

```bash
curl -X GET http://localhost:8081/admin/system/health \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq
```

**Ожидаемый ответ (403 Forbidden):**
```json
{
  "error": "Super admin access required",
  "message": "This action requires super administrator privileges",
  "request_id": "req-127"
}
```

## Batch тестирование

### Создать несколько пользователей

```bash
#!/bin/bash

for i in {1..5}; do
  curl -X POST http://localhost:8081/admin/users \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"email\": \"user$i@example.com\",
      \"name\": \"User $i\",
      \"password\": \"password123\",
      \"role\": \"employee\",
      \"position\": \"Developer $i\"
    }"
  echo "Created user $i"
  sleep 1
done
```

### Проверить список пользователей

```bash
curl -X GET "http://localhost:8081/admin/users?limit=20" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" | jq '.users | length'
```

## Логи и мониторинг

Все админские действия логируются с деталями:
- ID администратора
- Тип действия
- Время выполнения
- IP адрес
- User Agent
- Результат операции

Проверьте логи сервиса для отслеживания всех админских операций.