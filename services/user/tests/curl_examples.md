# User Service API Testing

## Предварительные требования

1. Запустить PostgreSQL и Redis:
```bash
make up  # Запускает docker-compose с PostgreSQL и Redis
```

2. Настроить переменные окружения (.env файл):
```bash
DATABASE_URL=postgres://tachyon_user:tachyon_password@localhost:5432/tachyon_messenger?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=your-super-secret-jwt-key-here
USER_SERVICE_PORT=8081
```

3. Запустить User Service:
```bash
cd services/user
go run main.go
```

## Тестирование через cURL

### 1. Health Check
```bash
curl -X GET http://localhost:8081/health \
  -H "Content-Type: application/json" | jq
```

**Ожидаемый ответ:**
```json
{
  "status": "healthy",
  "service": "user-service",
  "timestamp": "2025-01-20T10:30:00Z",
  "version": "1.0.0"
}
```

### 2. Регистрация пользователя

#### Успешная регистрация:
```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "name": "John Doe",
    "password": "password123",
    "role": "employee",
    "position": "Software Engineer",
    "phone": "+1234567890"
  }' | jq
```

#### Регистрация с отделом:
```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "jane.smith@example.com",
    "name": "Jane Smith",
    "password": "securepass456",
    "role": "manager",
    "department_id": 1,
    "position": "Team Lead",
    "phone": "+1987654321"
  }' | jq
```

#### Регистрация администратора:
```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "name": "Admin User",
    "password": "adminpass789",
    "role": "admin"
  }' | jq
```

**Ожидаемый ответ (201 Created):**
```json
{
  "message": "User registered successfully",
  "user": {
    "id": 1,
    "email": "john.doe@example.com",
    "name": "John Doe",
    "role": "employee",
    "status": "offline",
    "position": "Software Engineer",
    "phone": "+1234567890",
    "is_active": true,
    "created_at": "2025-01-20T10:30:00Z",
    "updated_at": "2025-01-20T10:30:00Z"
  },
  "request_id": "req-123-456"
}
```

### 3. Вход в систему

#### Успешный вход:
```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "password": "password123"
  }' | jq
```

**Ожидаемый ответ (200 OK):**
```json
{
  "message": "Login successful",
  "user": {
    "id": 1,
    "email": "john.doe@example.com",
    "name": "John Doe",
    "role": "employee",
    "status": "online",
    "position": "Software Engineer",
    "phone": "+1234567890",
    "last_active_at": "2025-01-20T10:35:00Z",
    "is_active": true
  },
  "tokens": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
  },
  "request_id": "req-789-012"
}
```

### 4. Использование JWT токена

Сохраните access_token из ответа логина:
```bash
export ACCESS_TOKEN="eyJhbGciOiJIUzI1NiIs..."
```

#### Получение информации о пользователе:
```bash
curl -X GET http://localhost:8081/api/v1/users/1 \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" | jq
```

#### Получение списка пользователей:
```bash
curl -X GET "http://localhost:8081/api/v1/users?limit=10&offset=0" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" | jq
```

#### Обновление пользователя:
```bash
curl -X PUT http://localhost:8081/api/v1/users/1 \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Updated Doe",
    "status": "busy",
    "position": "Senior Software Engineer"
  }' | jq
```

### 5. Тестирование ошибок

#### Неверный email при регистрации:
```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "invalid-email",
    "name": "Test User",
    "password": "password123"
  }' | jq
```

#### Слабый пароль:
```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "name": "Test User",
    "password": "123"
  }' | jq
```

#### Дублирующий email:
```bash
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "name": "Another User",
    "password": "password123"
  }' | jq
```

#### Неверные учетные данные при входе:
```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "password": "wrongpassword"
  }' | jq
```

#### Доступ без токена:
```bash
curl -X GET http://localhost:8081/api/v1/users/1 \
  -H "Content-Type: application/json" | jq
```

#### Неверный токен:
```bash
curl -X GET http://localhost:8081/api/v1/users/1 \
  -H "Authorization: Bearer invalid-token" \
  -H "Content-Type: application/json" | jq
```

## Postman Testing

### Настройка окружения в Postman:

1. Создайте новое окружение "User Service"
2. Добавьте переменные:
   - `base_url`: `http://localhost:8081`
   - `access_token`: (будет установлен автоматически)

### Коллекция запросов:

1. **Health Check**: `GET {{base_url}}/health`

2. **Register User**: `POST {{base_url}}/auth/register`
   ```json
   {
     "email": "test@example.com",
     "name": "Test User",
     "password": "password123",
     "role": "employee"
   }
   ```

3. **Login**: `POST {{base_url}}/auth/login`
   ```json
   {
     "email": "test@example.com",
     "password": "password123"
   }
   ```
   
   **Post-response Script:**
   ```javascript
   if (pm.response.code === 200) {
     const response = pm.response.json();
     pm.environment.set("access_token", response.tokens.access_token);
   }
   ```

4. **Get User**: `GET {{base_url}}/api/v1/users/1`
   - Headers: `Authorization: Bearer {{access_token}}`

5. **Get Users List**: `GET {{base_url}}/api/v1/users?limit=10&offset=0`
   - Headers: `Authorization: Bearer {{access_token}}`

### Тестирование в Postman:

1. Запустите Health Check
2. Зарегистрируйте пользователя
3. Войдите в систему (токен сохранится автоматически)
4. Тестируйте защищенные endpoints с токеном
5. Проверьте ошибки без токена или с неверными данными

## Проверка базы данных

Подключитесь к PostgreSQL для проверки данных:
```bash
docker exec -it tachyon-postgres psql -U tachyon_user -d tachyon_messenger
```

Полезные SQL запросы:
```sql
-- Проверить созданные таблицы
\dt

-- Посмотреть структуру таблицы users
\d users

-- Посмотреть всех пользователей
SELECT id, email, name, role, status, is_active, created_at FROM users;

-- Посмотреть отделы
SELECT * FROM departments;

-- Проверить связи
SELECT u.name, u.email, d.name as department 
FROM users u 
LEFT JOIN departments d ON u.department_id = d.id;
```