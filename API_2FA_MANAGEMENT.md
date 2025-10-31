# API для управления 2FA в TaxionDashboard

## Обзор

API для управления двухфакторной аутентификацией (2FA) пользователей из админ-панели.

## Требования

- Роль: **super_admin** (ТОЛЬКО супер-администратор)
- Аутентификация: JWT токен в заголовке `Authorization: Bearer <token>`

**⚠️ ВАЖНО:** Обычные администраторы (роль `admin`) НЕ могут управлять 2FA. Только `super_admin` имеет доступ к этому функционалу.

## Endpoints

### 1. Включение/выключение 2FA для пользователя

**Endpoint:** `PUT /api/v1/admin/users/:id/2fa`

**Описание:** Включает или выключает требование 2FA для конкретного пользователя.

**Headers:**
```
Authorization: Bearer <admin_jwt_token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "two_factor_enabled": true
}
```

**Parameters:**
- `two_factor_enabled` (boolean, required): `true` - включить 2FA, `false` - выключить 2FA

**Response (Success - 200):**
```json
{
  "message": "2FA status updated successfully",
  "user": {
    "id": 17,
    "email": "user@example.com",
    "name": "John Doe",
    "role": "employee",
    "status": "offline",
    "is_active": true,
    "two_factor_enabled": true,
    "created_at": "2025-10-31T14:44:41.798258Z",
    "updated_at": "2025-10-31T14:56:07.123456Z"
  },
  "request_id": "7ba9b1af-f755-4802-9adb-c8c27b42113e"
}
```

**Response (Error - 401 Unauthorized):**
```json
{
  "error": "Unauthorized",
  "request_id": "abc-123"
}
```

**Response (Error - 403 Forbidden - Non-Super Admin):**
```json
{
  "error": "Only super administrators can manage 2FA settings",
  "request_id": "abc-123"
}
```

**Примечание:** Эта ошибка возникает если обычный администратор (роль `admin`) пытается управлять 2FA. Только `super_admin` имеет доступ.

**Response (Error - 404 Not Found):**
```json
{
  "error": "User not found",
  "request_id": "abc-123"
}
```

## Примеры использования

### JavaScript/Fetch (для TaxionDashboard)

```javascript
/**
 * Включает 2FA для пользователя
 * @param {number} userId - ID пользователя
 * @param {boolean} enabled - Включить (true) или выключить (false) 2FA
 * @param {string} adminToken - JWT токен администратора
 */
async function updateUser2FA(userId, enabled, adminToken) {
  const response = await fetch(`http://localhost:8081/api/v1/admin/users/${userId}/2fa`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${adminToken}`,
    },
    body: JSON.stringify({
      two_factor_enabled: enabled
    }),
  });

  const data = await response.json();

  if (!response.ok) {
    throw new Error(data.error || 'Failed to update 2FA status');
  }

  return data;
}

// Пример использования в React компоненте
function User2FAToggle({ user, adminToken, onUpdate }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const handleToggle = async (enabled) => {
    setLoading(true);
    setError(null);

    try {
      const result = await updateUser2FA(user.id, enabled, adminToken);
      console.log('2FA updated:', result);
      onUpdate(result.user);
      alert(`2FA ${enabled ? 'включена' : 'выключена'} для ${user.name}`);
    } catch (err) {
      console.error('Error updating 2FA:', err);
      setError(err.message);
      alert(`Ошибка: ${err.message}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="user-2fa-control">
      <h3>{user.name} ({user.email})</h3>
      <div className="toggle-container">
        <label>
          <input
            type="checkbox"
            checked={user.two_factor_enabled}
            onChange={(e) => handleToggle(e.target.checked)}
            disabled={loading}
          />
          2FA {user.two_factor_enabled ? 'включена' : 'выключена'}
        </label>
      </div>
      {loading && <p>Обновление...</p>}
      {error && <p className="error">{error}</p>}
    </div>
  );
}
```

### cURL (для тестирования)

```bash
# 1. Получить токен администратора (через 2FA)
curl -X POST http://localhost:8081/api/v1/auth/2fa/send \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "admin123"
  }'

# Получите код из email и проверьте его
curl -X POST http://localhost:8081/api/v1/auth/2fa/verify \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "code": "123456"
  }'
# Сохраните access_token из ответа

# 2. Включить 2FA для пользователя с ID=17
TOKEN="<your_admin_access_token>"
curl -X PUT http://localhost:8081/api/v1/admin/users/17/2fa \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "two_factor_enabled": true
  }'

# 3. Выключить 2FA для пользователя с ID=17
curl -X PUT http://localhost:8081/api/v1/admin/users/17/2fa \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "two_factor_enabled": false
  }'

# 4. Получить список всех пользователей с их 2FA статусом
curl -X GET http://localhost:8081/api/v1/admin/users \
  -H "Authorization: Bearer $TOKEN"
```

## Интеграция с TaxionDashboard

### Рекомендуемая архитектура

1. **Список пользователей с индикатором 2FA**
   ```jsx
   <UserList>
     {users.map(user => (
       <UserCard key={user.id}>
         <UserInfo name={user.name} email={user.email} />
         <TwoFactorBadge enabled={user.two_factor_enabled} />
         <Toggle2FAButton
           userId={user.id}
           currentStatus={user.two_factor_enabled}
           onToggle={handle2FAToggle}
         />
       </UserCard>
     ))}
   </UserList>
   ```

2. **Модальное окно подтверждения**
   ```jsx
   function Confirm2FAChange({ user, newStatus, onConfirm, onCancel }) {
     return (
       <Modal>
         <h3>Изменить настройку 2FA?</h3>
         <p>
           {newStatus
             ? `Включить 2FA для ${user.name}? После этого пользователю потребуется вводить код из email при каждом входе.`
             : `Выключить 2FA для ${user.name}? После этого пользователь сможет входить только с паролем.`
           }
         </p>
         <Button onClick={onConfirm}>Подтвердить</Button>
         <Button onClick={onCancel}>Отмена</Button>
       </Modal>
     );
   }
   ```

3. **API Service (для централизованного управления запросами)**
   ```javascript
   // services/adminApi.js
   const API_BASE = 'http://localhost:8081/api/v1';

   class AdminAPI {
     constructor(token) {
       this.token = token;
       this.headers = {
         'Content-Type': 'application/json',
         'Authorization': `Bearer ${token}`,
       };
     }

     async getUsers(limit = 20, offset = 0) {
       const response = await fetch(
         `${API_BASE}/admin/users?limit=${limit}&offset=${offset}`,
         { headers: this.headers }
       );
       return this.handleResponse(response);
     }

     async updateUser2FA(userId, enabled) {
       const response = await fetch(
         `${API_BASE}/admin/users/${userId}/2fa`,
         {
           method: 'PUT',
           headers: this.headers,
           body: JSON.stringify({ two_factor_enabled: enabled }),
         }
       );
       return this.handleResponse(response);
     }

     async handleResponse(response) {
       const data = await response.json();
       if (!response.ok) {
         throw new Error(data.error || 'API request failed');
       }
       return data;
     }
   }

   export default AdminAPI;
   ```

## Поле `two_factor_enabled` в User объекте

После обновления все API endpoints, возвращающие информацию о пользователях, теперь включают поле:

```javascript
{
  "id": 17,
  "email": "user@example.com",
  "name": "John Doe",
  "role": "employee",
  "status": "offline",
  "is_active": true,
  "two_factor_enabled": false,  // <-- Новое поле
  "created_at": "2025-10-31T14:44:41.798258Z",
  "updated_at": "2025-10-31T14:56:07.123456Z"
}
```

Это поле можно использовать для:
- Отображения бейджа "2FA включена" рядом с пользователем
- Фильтрации пользователей по статусу 2FA
- Условного рендеринга кнопок управления

## Логика работы 2FA после включения

1. **2FA выключена** (`two_factor_enabled: false`)
   - Пользователь входит с помощью `POST /auth/login` (email + password)
   - Получает токены сразу

2. **2FA включена** (`two_factor_enabled: true`)
   - Пользователь НЕ может использовать `POST /auth/login`
   - Должен использовать:
     - `POST /auth/2fa/send` - отправка кода на email
     - `POST /auth/2fa/verify` - проверка кода и получение токенов

## Безопасность

- ✅ Только **admin** и **super_admin** могут изменять 2FA статус
- ✅ Все действия логируются с request_id для аудита
- ✅ Middleware `LogAdminAction` записывает все изменения
- ✅ JWT токен проверяется на каждый запрос
- ✅ Изменения применяются мгновенно - следующий вход пользователя потребует 2FA

## Миграция базы данных

Поле `two_factor_enabled` автоматически добавляется в таблицу `users` при первом запуске обновленного сервиса.

Структура:
```sql
ALTER TABLE users ADD COLUMN two_factor_enabled BOOLEAN NOT NULL DEFAULT false;
```

## Troubleshooting

**Проблема:** 401 Unauthorized при запросе

**Решение:**
1. Проверьте что токен валидный и не истёк
2. Убедитесь что токен передаётся в заголовке `Authorization: Bearer <token>`
3. Проверьте что пользователь имеет роль `admin` или `super_admin`

**Проблема:** 403 Forbidden

**Решение:**
- Пользователь не имеет прав администратора
- Обновите роль пользователя через `PUT /api/v1/admin/users/:id/role`

**Проблема:** Пользователь не может войти после включения 2FA

**Решение:**
- Это ожидаемое поведение
- Пользователь должен использовать flow `/auth/2fa/send` → `/auth/2fa/verify`
- Убедитесь что SMTP настроен правильно для отправки кодов

---

**Версия:** 1.0.0
**Дата обновления:** 31 октября 2025
