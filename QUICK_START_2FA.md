# Быстрый старт: 2FA через Email

## Шаг 1: Настройте SMTP

Скопируйте пример конфигурации:
```bash
cp .env.2fa.example .env.2fa
```

Отредактируйте `.env` или `.env.2fa` и добавьте SMTP настройки:

### Вариант A: Яндекс.Почта (Бесплатно)

```env
SMTP_HOST=smtp.yandex.ru
SMTP_PORT=587
SMTP_USER=noreply@yourdomain.ru
SMTP_PASSWORD=ваш_пароль_приложения
SMTP_FROM_EMAIL=noreply@yourdomain.ru
SMTP_FROM_NAME=Tachyon Messenger
```

**Как получить пароль приложения Яндекса:**
1. Перейдите на https://id.yandex.ru/security/app-passwords
2. Создайте новый пароль приложения
3. Скопируйте его в SMTP_PASSWORD

### Вариант B: MailHog (Для тестирования)

```bash
# Запустите MailHog
docker run -d -p 1025:1025 -p 8025:8025 mailhog/mailhog
```

```env
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=test
SMTP_PASSWORD=test
SMTP_FROM_EMAIL=noreply@localhost
```

Откройте http://localhost:8025 чтобы видеть письма.

## Шаг 2: Запустите сервисы

```bash
docker-compose build user-service
docker-compose up -d
```

## Шаг 3: Проверьте что всё работает

### Тест 1: Отправить код

```bash
curl -X POST http://localhost:8080/api/v1/auth/2fa/send \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "admin123"
  }'
```

**Ожидаемый результат:**
```json
{
  "message": "Verification code sent to your email",
  "request_id": "..."
}
```

### Тест 2: Проверить код

Получите код из email (или из MailHog) и выполните:

```bash
curl -X POST http://localhost:8080/api/v1/auth/2fa/verify \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "code": "123456"
  }'
```

**Ожидаемый результат:**
```json
{
  "message": "Login successful",
  "user": { ... },
  "tokens": { ... }
}
```

## Готово! 🎉

Теперь ваша система поддерживает 2FA через email.

## Endpoints

- `POST /auth/2fa/send` - отправить код
- `POST /auth/2fa/verify` - проверить код

## Troubleshooting

**Проблема:** Письма не отправляются

**Решение:**
1. Проверьте логи: `docker logs tachyon-user-service`
2. Убедитесь что SMTP настройки правильные
3. Для Яндекса: используйте пароль приложения, не основной пароль

**Проблема:** Код недействителен

**Решение:**
- Код действителен только 5 минут
- Код можно использовать только один раз
- Проверьте что email совпадает

## Подробная документация

См. [2FA_SETUP.md](./2FA_SETUP.md) для полной документации.
