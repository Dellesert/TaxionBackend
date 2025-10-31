# Настройка двухфакторной аутентификации (2FA) через Email

## Обзор

Система поддерживает двухфакторную аутентификацию через отправку кодов на email пользователя.
Код действителен в течение 5 минут и может быть использован только один раз.

## Требования

- SMTP сервер для отправки email (рекомендуется UniSender или Яндекс.Почта)
- Настроенные переменные окружения для SMTP

## Конфигурация SMTP

Добавьте следующие переменные в ваш `.env` файл:

```env
# SMTP Configuration для отправки 2FA кодов
SMTP_HOST=smtp.yandex.ru              # Хост SMTP сервера
SMTP_PORT=587                          # Порт (587 для TLS, 465 для SSL)
SMTP_USER=noreply@yourdomain.ru       # SMTP пользователь (email)
SMTP_PASSWORD=your_app_password        # Пароль приложения
SMTP_FROM_EMAIL=noreply@yourdomain.ru # Email отправителя
SMTP_FROM_NAME=Tachyon Messenger      # Имя отправителя
```

### Рекомендуемые SMTP провайдеры

#### 1. **UniSender** (Рекомендуется для коммерческого использования)
- Бесплатно: 14,000 писем/месяц
- Хорошая доставляемость
- Российский сервис

```env
SMTP_HOST=smtp.unisender.com
SMTP_PORT=587
SMTP_USER=your_api_key
SMTP_PASSWORD=your_api_secret
```

#### 2. **Яндекс.Почта для доменов** (Бесплатно)
- Бесплатно: ~500 писем/день
- Требуется свой домен
- Полностью российский сервис

```env
SMTP_HOST=smtp.yandex.ru
SMTP_PORT=587
SMTP_USER=noreply@yourdomain.ru
SMTP_PASSWORD=your_app_password
```

**Важно для Яндекса:**
1. Подключите домен к Яндекс.Почте
2. Создайте почтовый ящик (например, noreply@yourdomain.ru)
3. В настройках почты включите "Доступ по протоколу IMAP"
4. Создайте [пароль приложения](https://id.yandex.ru/security/app-passwords)

## API Endpoints

### 1. Отправка 2FA кода

**Endpoint:** `POST /auth/2fa/send` или `POST /api/v1/auth/2fa/send`

**Описание:** Проверяет учетные данные и отправляет 6-значный код на email пользователя.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "userpassword"
}
```

**Response (Success - 200):**
```json
{
  "message": "Verification code sent to your email",
  "request_id": "req-123-456"
}
```

**Response (Error - 401):**
```json
{
  "error": "Invalid email or password",
  "request_id": "req-123-456"
}
```

### 2. Проверка 2FA кода

**Endpoint:** `POST /auth/2fa/verify` или `POST /api/v1/auth/2fa/verify`

**Описание:** Проверяет 2FA код и завершает процесс авторизации.

**Request:**
```json
{
  "email": "user@example.com",
  "code": "123456"
}
```

**Response (Success - 200 - JWT Mode):**
```json
{
  "message": "Login successful",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe",
    "role": "employee",
    "status": "online"
  },
  "auth_mode": "jwt",
  "tokens": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 900
  },
  "request_id": "req-123-456"
}
```

**Response (Success - 200 - Session Mode):**
```json
{
  "message": "Login successful",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "name": "John Doe",
    "role": "employee",
    "status": "online"
  },
  "auth_mode": "session",
  "session": {
    "session_id": "sess-abc-def",
    "expires_at": 1234567890
  },
  "request_id": "req-123-456"
}
```

**Response (Error - 401):**
```json
{
  "error": "Invalid or expired verification code",
  "request_id": "req-123-456"
}
```

## Процесс использования

### Пример потока авторизации с 2FA:

```
1. Пользователь → POST /auth/2fa/send
   {
     "email": "user@example.com",
     "password": "password123"
   }

2. Система проверяет пароль ✓

3. Система генерирует 6-значный код и отправляет на email ✉️

4. Пользователь получает email с кодом (действителен 5 минут)

5. Пользователь → POST /auth/2fa/verify
   {
     "email": "user@example.com",
     "code": "123456"
   }

6. Система проверяет код ✓

7. Система возвращает токены/сессию 🎉
```

### Пример использования (JavaScript/Fetch):

```javascript
// Шаг 1: Отправить код
async function send2FACode(email, password) {
  const response = await fetch('http://localhost:8080/api/v1/auth/2fa/send', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password }),
  });

  const data = await response.json();

  if (!response.ok) {
    throw new Error(data.error || 'Failed to send code');
  }

  return data;
}

// Шаг 2: Проверить код
async function verify2FACode(email, code) {
  const response = await fetch('http://localhost:8080/api/v1/auth/2fa/verify', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, code }),
  });

  const data = await response.json();

  if (!response.ok) {
    throw new Error(data.error || 'Invalid code');
  }

  // Сохранить токены
  if (data.tokens) {
    localStorage.setItem('access_token', data.tokens.access_token);
    localStorage.setItem('refresh_token', data.tokens.refresh_token);
  }

  return data;
}

// Использование
try {
  // Запросить код
  await send2FACode('user@example.com', 'password123');
  alert('Код отправлен на ваш email!');

  // Пользователь вводит код из email
  const code = prompt('Введите код из email:');

  // Проверить код
  const result = await verify2FACode('user@example.com', code);
  console.log('Logged in:', result.user);
} catch (error) {
  console.error('Error:', error.message);
}
```

## Безопасность

### Особенности реализации:

1. **Срок действия кода:** 5 минут
2. **Одноразовое использование:** После проверки код становится недействительным
3. **Криптографически стойкий генератор:** Используется `crypto/rand`
4. **Защита от перебора:** Код удаляется после первой успешной проверки
5. **Хранение:** Коды хранятся в БД с индексами по email и времени истечения

### Рекомендации:

- ✅ Используйте HTTPS в production
- ✅ Настройте rate limiting на endpoints 2FA
- ✅ Храните SMTP пароли в секретах (не в .env файлах в репозитории)
- ✅ Регулярно очищайте истекшие коды (автоматическая очистка при следующем запросе)
- ✅ Логируйте все попытки аутентификации

## Миграции базы данных

Таблица для хранения 2FA кодов создается автоматически при запуске сервиса:

```sql
CREATE TABLE two_factor_codes (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    code VARCHAR(6) NOT NULL,
    email VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT false,
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_two_factor_codes_email ON two_factor_codes(email);
CREATE INDEX idx_two_factor_codes_user_id ON two_factor_codes(user_id);
CREATE INDEX idx_two_factor_codes_expires_at ON two_factor_codes(expires_at);
```

## Тестирование

### Локальное тестирование (без SMTP):

Для тестирования без реального SMTP сервера можно использовать [MailHog](https://github.com/mailhog/MailHog):

```bash
# Запустить MailHog
docker run -d -p 1025:1025 -p 8025:8025 mailhog/mailhog

# Настроить .env
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=test
SMTP_PASSWORD=test
SMTP_FROM_EMAIL=noreply@localhost

# Открыть веб-интерфейс MailHog
# http://localhost:8025
```

### Тестирование с curl:

```bash
# 1. Отправить код
curl -X POST http://localhost:8080/api/v1/auth/2fa/send \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'

# 2. Проверить код
curl -X POST http://localhost:8080/api/v1/auth/2fa/verify \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "code": "123456"
  }'
```

## Troubleshooting

### Проблема: Письма не отправляются

**Решение:**
1. Проверьте SMTP настройки в `.env`
2. Убедитесь что SMTP_HOST и SMTP_PORT правильные
3. Проверьте логи: `docker logs tachyon-user-service`
4. Для Яндекса: убедитесь что используете пароль приложения, а не основной пароль

### Проблема: Письма попадают в спам

**Решение:**
1. Настройте SPF, DKIM, DMARC для вашего домена
2. Используйте проверенный SMTP сервис (UniSender, Yandex)
3. Используйте реальный домен, не бесплатные email сервисы

### Проблема: Код истекает слишком быстро

**Решение:**
Измените время жизни кода в [services/user/usecase/twofa_usecase.go](services/user/usecase/twofa_usecase.go:82):

```go
ExpiresAt: time.Now().Add(10 * time.Minute), // Изменить с 5 на 10 минут
```

## Поддержка

Если у вас возникли вопросы или проблемы:
1. Проверьте логи сервиса
2. Убедитесь что все переменные окружения настроены
3. Протестируйте SMTP подключение отдельно

---

**Версия:** 1.0.0
**Дата обновления:** 31 октября 2025
