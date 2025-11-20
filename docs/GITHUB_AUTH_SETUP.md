# Настройка авторизации в GitHub Container Registry

Для работы с приватными образами в GitHub Container Registry нужна авторизация по токену.

## Быстрая настройка (5 минут)

### Шаг 1: Создание Personal Access Token

1. Откройте [GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)](https://github.com/settings/tokens)

2. Нажмите **"Generate new token (classic)"**

3. Заполните форму:
   - **Note**: `GHCR Access for Production Server` (или любое описание)
   - **Expiration**: `No expiration` или выберите срок
   - **Select scopes**:
     - ✅ `read:packages` - **обязательно** (для pull образов)
     - ✅ `write:packages` - опционально (для push, если нужно)
     - ✅ `delete:packages` - опционально (для удаления старых версий)

4. Нажмите **"Generate token"**

5. **ВАЖНО**: Скопируйте токен и сохраните в безопасном месте! Он будет показан только один раз.

Токен выглядит так: `ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

### Шаг 2: Настройка на сервере

Есть **3 способа** настройки. Выберите наиболее подходящий:

---

## Способ 1: Файл с credentials (рекомендуется) ⭐

Самый безопасный и удобный способ.

### Создание файла

```bash
# Создайте файл с credentials
cat > ~/.github-credentials << 'EOF'
GITHUB_TOKEN=ghp_your_actual_token_here
GITHUB_USERNAME=your-github-username
EOF

# Ограничьте права доступа (только владелец может читать)
chmod 600 ~/.github-credentials

# Проверьте права
ls -la ~/.github-credentials
# Должно быть: -rw------- (600)
```

### Использование

Скрипт деплоя автоматически загрузит credentials из файла:

```bash
./scripts/deployment/deploy.sh deploy
```

Или вручную:

```bash
# Загрузить credentials
source ~/.github-credentials

# Залогиниться
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

### Преимущества:
- ✅ Безопасно (файл с ограниченными правами)
- ✅ Удобно (не нужно каждый раз вводить)
- ✅ Автоматически используется скриптом
- ✅ Не хранится в истории команд

---

## Способ 2: Переменные окружения в .bashrc/.zshrc

Удобно для постоянного использования на dev машине.

### Настройка

```bash
# Откройте ~/.bashrc (для bash) или ~/.zshrc (для zsh)
nano ~/.bashrc

# Добавьте в конец файла
export GITHUB_TOKEN="ghp_your_actual_token_here"
export GITHUB_USERNAME="your-github-username"

# Сохраните и перезагрузите shell
source ~/.bashrc
```

### Использование

```bash
# Переменные уже доступны
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin

# Или просто запустите скрипт
./scripts/deployment/deploy.sh deploy
```

### Преимущества:
- ✅ Всегда доступно в терминале
- ✅ Не нужно создавать отдельный файл

### Недостатки:
- ⚠️ Видно в переменных окружения всех процессов
- ⚠️ Может быть небезопасно на shared сервере

---

## Способ 3: Ручной ввод каждый раз

Самый безопасный, но неудобный.

```bash
# Установите переменные вручную
export GITHUB_TOKEN="ghp_your_token"
export GITHUB_USERNAME="your-username"

# Залогиньтесь
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

После закрытия терминала credentials будут удалены.

### Преимущества:
- ✅ Максимально безопасно
- ✅ Токен не хранится на диске

### Недостатки:
- ❌ Нужно вводить каждый раз
- ❌ Неудобно для автоматизации

---

## Проверка авторизации

### Проверить что залогинены

```bash
# Проверка через docker info
docker info | grep -A 3 "ghcr.io"

# Или проверка через config
cat ~/.docker/config.json | grep ghcr.io
```

### Тест pull образа

```bash
# Попробуйте скачать образ
docker pull ghcr.io/YOUR_USERNAME/tachyon-user:latest

# Если успешно - авторизация работает!
```

---

## Использование с скриптом деплоя

Скрипт [scripts/deployment/deploy.sh](scripts/deployment/deploy.sh) автоматически:

1. Проверяет, залогинены ли вы в GHCR
2. Если нет - пытается загрузить credentials из `~/.github-credentials`
3. Если файла нет - проверяет переменные окружения `$GITHUB_TOKEN` и `$GITHUB_USERNAME`
4. Если credentials найдены - логинится автоматически
5. Если не найдены - показывает инструкцию

### Пример использования

```bash
# Первый запуск - скрипт попросит credentials
./scripts/deployment/deploy.sh deploy

# Последующие запуски - автоматический логин
./scripts/deployment/deploy.sh deploy
```

---

## Безопасность токенов

### Что делать, если токен утек

1. Немедленно удалите токен:
   - [GitHub Settings → Tokens](https://github.com/settings/tokens)
   - Найдите токен и нажмите **Delete**

2. Создайте новый токен (см. Шаг 1)

3. Обновите credentials на сервере:
```bash
nano ~/.github-credentials  # Обновите GITHUB_TOKEN
```

### Best Practices

✅ **Делайте:**
- Используйте отдельные токены для разных серверов
- Указывайте срок действия токена (expiration)
- Давайте минимально необходимые права (только `read:packages` для production)
- Используйте файл с правами 600
- Регулярно ротируйте токены (каждые 3-6 месяцев)

❌ **Не делайте:**
- Не коммитьте токены в git
- Не храните в публичных местах
- Не используйте токены с `admin` правами для CI/CD
- Не создавайте токены без expiration для временных задач

---

## Альтернативные способы

### Docker Credential Helper (advanced)

Для максимальной безопасности используйте credential helper:

```bash
# Установка (Ubuntu/Debian)
sudo apt-get install pass gnupg2

# Или для macOS
brew install docker-credential-helper

# Настройка
# Docker автоматически использует system keychain
# для хранения credentials в зашифрованном виде
```

После настройки credentials хранятся в system keychain (macOS Keychain, Linux pass).

---

## Troubleshooting

### Ошибка: "unauthorized: unauthenticated"

**Причина**: Не залогинены в GHCR

**Решение**:
```bash
# Залогиниться
source ~/.github-credentials
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

### Ошибка: "denied: permission_denied"

**Причина**:
- Неправильный токен
- Недостаточно прав у токена
- Пакет не существует или приватный

**Решение**:
1. Проверьте токен имеет права `read:packages`
2. Проверьте username правильный
3. Проверьте пакет существует: https://github.com/YOUR_USERNAME?tab=packages

### Ошибка: "Error saving credentials"

**Причина**: Проблемы с docker credential store

**Решение**:
```bash
# Очистите старые credentials
rm ~/.docker/config.json

# Залогиниться заново
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

### Токен не работает

**Проверьте**:
1. Токен не истек (check expiration date)
2. Токен имеет правильные scopes
3. Копия токена полная (нет пробелов в начале/конце)

```bash
# Проверьте токен вручную
curl -H "Authorization: Bearer $GITHUB_TOKEN" https://ghcr.io/v2/
# Должно вернуть 200 OK
```

---

## Ротация токенов

Рекомендуется менять токены каждые 3-6 месяцев.

### Процесс ротации

```bash
# 1. Создайте новый токен на GitHub

# 2. Обновите на всех серверах
nano ~/.github-credentials
# Замените GITHUB_TOKEN на новый

# 3. Перелогиньтесь
docker logout ghcr.io
source ~/.github-credentials
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin

# 4. Удалите старый токен на GitHub
```

---

## CI/CD (GitHub Actions)

Для GitHub Actions токен не нужен! Используйте встроенный `GITHUB_TOKEN`:

```yaml
- name: Login to GHCR
  uses: docker/login-action@v3
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}  # Автоматически доступен
```

---

## Дополнительные ресурсы

- [GitHub Packages Documentation](https://docs.github.com/en/packages)
- [Working with Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Managing PATs](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
