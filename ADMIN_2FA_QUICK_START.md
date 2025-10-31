# Быстрый старт: Управление 2FA для админов

## 🎯 Что добавлено

Теперь супер-администраторы могут включать или выключать 2FA для любого пользователя через API!

## 📋 Возможности

- ✅ Включение/выключение 2FA для пользователей через админ-панель
- ✅ Новое поле `two_factor_enabled` в профиле пользователя
- ✅ API endpoint: `PUT /api/v1/admin/users/:id/2fa`
- ✅ Полная интеграция с TaxionDashboard
- ✅ Автоматическая миграция базы данных

## 🚀 Быстрый тест

### Способ 1: Тестовый HTML интерфейс

1. Откройте в браузере файл `test_2fa_admin_panel.html`
2. Войдите как админ через 2FA:
   - Email: `mishajackson@inbox.ru`
   - Password: `Test123!@#`
   - Введите код из email
3. Увидите список пользователей с переключателями 2FA
4. Включите/выключите 2FA для любого пользователя одним кликом!

### Способ 2: cURL команды

```bash
# 1. Войти как админ (через 2FA)
curl -X POST http://localhost:8081/api/v1/auth/2fa/send \
  -H "Content-Type: application/json" \
  -d '{"email": "mishajackson@inbox.ru", "password": "Test123!@#"}'

# Получите код из email/логов и проверьте его
curl -X POST http://localhost:8081/api/v1/auth/2fa/verify \
  -H "Content-Type: application/json" \
  -d '{"email": "mishajackson@inbox.ru", "code": "123456"}'

# Сохраните access_token из ответа
TOKEN="<ваш_access_token>"

# 2. Получить список пользователей
curl -X GET "http://localhost:8081/api/v1/admin/users" \
  -H "Authorization: Bearer $TOKEN"

# 3. Включить 2FA для пользователя (замените ID)
curl -X PUT "http://localhost:8081/api/v1/admin/users/17/2fa" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"two_factor_enabled": true}'

# 4. Выключить 2FA для пользователя
curl -X PUT "http://localhost:8081/api/v1/admin/users/17/2fa" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"two_factor_enabled": false}'
```

## 📖 Детальная документация

См. [`API_2FA_MANAGEMENT.md`](./API_2FA_MANAGEMENT.md) для:
- Полное описание API endpoints
- Примеры интеграции с React/Vue/Angular
- Образец кода для TaxionDashboard
- Примеры обработки ошибок
- Рекомендации по UI/UX

## 🔧 Технические детали

### Новые файлы

1. **Models** (`services/user/models/user.go`)
   - Добавлено поле `TwoFactorEnabled bool`
   - Добавлена структура `AdminUpdate2FARequest`

2. **Repository** (`services/user/repository/user_repository.go`)
   - Метод `UpdateTwoFactorStatus(userID uint, enabled bool)`

3. **Usecase** (`services/user/usecase/admin_usecase.go`)
   - Метод `UpdateUser2FAStatus(id uint, req *AdminUpdate2FARequest)`

4. **Handler** (`services/user/handlers/admin_handler.go`)
   - Метод `UpdateUser2FA(c *gin.Context)`

5. **Routes** (`services/user/main.go`)
   - `PUT /admin/users/:id/2fa` (с middleware AdminOnly)

### Миграция БД

Автоматически при первом запуске:
```sql
ALTER TABLE users ADD COLUMN two_factor_enabled BOOLEAN NOT NULL DEFAULT false;
```

## 🎨 Интеграция с TaxionDashboard

### Пример React компонента

```jsx
import React, { useState, useEffect } from 'react';
import { Switch } from '@mui/material';

function User2FAManager({ adminToken }) {
  const [users, setUsers] = useState([]);

  useEffect(() => {
    loadUsers();
  }, []);

  const loadUsers = async () => {
    const response = await fetch('http://localhost:8081/api/v1/admin/users', {
      headers: { 'Authorization': `Bearer ${adminToken}` }
    });
    const data = await response.json();
    setUsers(data.users);
  };

  const toggle2FA = async (userId, enabled) => {
    await fetch(`http://localhost:8081/api/v1/admin/users/${userId}/2fa`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${adminToken}`
      },
      body: JSON.stringify({ two_factor_enabled: enabled })
    });
    loadUsers();
  };

  return (
    <div>
      <h2>Управление 2FA</h2>
      {users.map(user => (
        <div key={user.id} style={{ padding: '10px', border: '1px solid #ccc', margin: '10px 0' }}>
          <div>
            <strong>{user.name}</strong> ({user.email})
          </div>
          <div>
            2FA:
            <Switch
              checked={user.two_factor_enabled}
              onChange={(e) => toggle2FA(user.id, e.target.checked)}
            />
            {user.two_factor_enabled ? 'Включена' : 'Выключена'}
          </div>
        </div>
      ))}
    </div>
  );
}

export default User2FAManager;
```

## 🔐 Безопасность

- Только роли `admin` и `super_admin` имеют доступ
- Все действия логируются для аудита
- JWT токены валидируются на каждый запрос
- Middleware `AdminOnlyMiddleware` блокирует неавторизованный доступ

## 📊 API Response

```json
{
  "message": "2FA status updated successfully",
  "user": {
    "id": 17,
    "email": "user@example.com",
    "name": "John Doe",
    "role": "employee",
    "two_factor_enabled": true,
    "created_at": "2025-10-31T14:44:41Z",
    "updated_at": "2025-10-31T15:00:00Z"
  },
  "request_id": "abc-123-def"
}
```

## ❓ FAQ

**Q: Что произойдёт если включить 2FA пользователю?**
A: При следующем входе пользователь будет обязан использовать 2FA flow (отправка кода на email + ввод кода).

**Q: Может ли пользователь сам отключить 2FA?**
A: Нет, только админы могут управлять этой настройкой.

**Q: Нужно ли перезапускать сервис после изменения 2FA?**
A: Нет, изменения применяются мгновенно.

**Q: Что если у пользователя нет доступа к email?**
A: Администратор может временно отключить 2FA для пользователя, чтобы тот смог войти.

## 🐛 Troubleshooting

**Проблема:** API возвращает 401 Unauthorized

**Решение:**
```bash
# Проверьте что токен валиден
curl -X GET "http://localhost:8081/api/v1/profile/me" \
  -H "Authorization: Bearer $TOKEN"

# Если токен истёк - получите новый через refresh token
```

**Проблема:** API возвращает 403 Forbidden

**Решение:**
```bash
# Проверьте роль пользователя
curl -X GET "http://localhost:8081/api/v1/profile/me" \
  -H "Authorization: Bearer $TOKEN"

# Роль должна быть "admin" или "super_admin"
```

**Проблема:** Пользователь не может войти после включения 2FA

**Решение:**
- Это ожидаемое поведение!
- Пользователь должен использовать `/auth/2fa/send` и `/auth/2fa/verify`
- Убедитесь что SMTP настроен корректно

## 📚 Связанная документация

- [2FA_SETUP.md](./2FA_SETUP.md) - Полная настройка 2FA
- [QUICK_START_2FA.md](./QUICK_START_2FA.md) - Быстрый старт 2FA
- [API_2FA_MANAGEMENT.md](./API_2FA_MANAGEMENT.md) - API документация

---

**Версия:** 1.0.0
**Дата:** 31 октября 2025
