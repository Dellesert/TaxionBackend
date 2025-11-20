# Просмотр Docker образов в GitHub Container Registry

## Где находятся образы

Ваши образы хранятся в **GitHub Container Registry (GHCR)** по адресу:
```
ghcr.io/YOUR_USERNAME/tachyon-SERVICE_NAME:TAG
```

Например:
```
ghcr.io/myusername/tachyon-user:v1.0.0
ghcr.io/myusername/tachyon-chat:latest
ghcr.io/myusername/tachyon-gateway:main
```

---

## 🌐 Просмотр через веб-интерфейс GitHub

### Способ 1: Через профиль

1. Откройте свой профиль на GitHub:
   ```
   https://github.com/YOUR_USERNAME
   ```

2. Перейдите на вкладку **"Packages"**:
   ```
   https://github.com/YOUR_USERNAME?tab=packages
   ```

3. Увидите список всех ваших образов:
   ```
   tachyon-user
   tachyon-chat
   tachyon-task
   tachyon-gateway
   ... и так далее
   ```

4. Кликните на любой образ, чтобы увидеть:
   - Все доступные теги (версии)
   - Размер образа
   - Дату создания
   - Digest (уникальный хеш)

### Способ 2: Через репозиторий

1. Откройте ваш репозиторий:
   ```
   https://github.com/YOUR_USERNAME/TaxionBack
   ```

2. Справа увидите секцию **"Packages"**

3. Там будут перечислены все образы из этого репозитория

---

## 💻 Просмотр через командную строку

### Список образов пользователя

GitHub API не предоставляет прямого способа посмотреть все образы через CLI, но можно использовать `gh` CLI:

```bash
# Установка gh CLI (если нет)
# Linux
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update
sudo apt install gh

# macOS
brew install gh

# Windows
winget install GitHub.cli
```

### Просмотр пакетов

```bash
# Логин
gh auth login

# Список пакетов (работает через API)
gh api /users/YOUR_USERNAME/packages?package_type=container

# Или через curl
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
     https://api.github.com/users/YOUR_USERNAME/packages?package_type=container
```

---

## 🐳 Просмотр локально через Docker

### Список локальных образов

```bash
# Все tachyon образы
docker images | grep tachyon

# Конкретный сервис
docker images | grep tachyon-user
```

### Pull образа для просмотра

```bash
# Авторизация
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin

# Pull конкретного образа
docker pull ghcr.io/YOUR_USERNAME/tachyon-user:v1.0.0

# Посмотреть детали
docker inspect ghcr.io/YOUR_USERNAME/tachyon-user:v1.0.0

# История слоёв
docker history ghcr.io/YOUR_USERNAME/tachyon-user:v1.0.0
```

---

## 📊 Просмотр через GitHub Actions

### После сборки

1. Откройте ваш репозиторий:
   ```
   https://github.com/YOUR_USERNAME/TaxionBack
   ```

2. Перейдите на вкладку **"Actions"**

3. Найдите последний запуск workflow **"Build and Push Docker Images"**

4. Откройте его и увидите:
   - Какие образы собрались
   - Время сборки
   - Логи сборки
   - Ошибки (если были)

5. В конце каждой job будет информация:
   ```
   Pushing to ghcr.io/username/tachyon-user:v1.0.0
   ✓ Image pushed successfully
   ```

---

## 🔍 Детальная информация об образе

### Через веб-интерфейс

После открытия пакета (например, `tachyon-user`), вы увидите:

**Теги (версии):**
```
latest        - 2 hours ago   - 145 MB
v1.0.0        - 1 day ago     - 145 MB
v1.0          - 1 day ago     - 145 MB
v1            - 1 day ago     - 145 MB
main          - 3 hours ago   - 145 MB
main-abc123   - 3 hours ago   - 145 MB
develop       - 1 week ago    - 144 MB
```

**Информация о теге:**
- **Digest:** `sha256:abc123...` (уникальный хеш)
- **OS/Arch:** `linux/amd64`
- **Compressed size:** 145 MB
- **Last published:** 2 hours ago

**Manifest:**
```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "size": 7023,
    "digest": "sha256:..."
  },
  "layers": [...]
}
```

---

## 🔐 Управление видимостью пакетов

По умолчанию пакеты **приватные**. Можно изменить:

### Через веб-интерфейс

1. Откройте пакет (например, `tachyon-user`)

2. Нажмите **"Package settings"** справа

3. В секции **"Danger Zone"** → **"Change package visibility"**

4. Выберите:
   - **Private** - только вы и collaborators (рекомендуется)
   - **Public** - все могут pull'ить

---

## 📋 Примеры URL

Замените `YOUR_USERNAME` на ваш GitHub username:

### Просмотр всех пакетов
```
https://github.com/YOUR_USERNAME?tab=packages
```

### Конкретный пакет
```
https://github.com/YOUR_USERNAME/tachyon-user
```

или через packages:
```
https://github.com/users/YOUR_USERNAME/packages/container/tachyon-user
```

### История GitHub Actions
```
https://github.com/YOUR_USERNAME/TaxionBack/actions
```

---

## 🎯 Быстрая проверка после push

После того как запушили код:

```bash
# 1. Push в GitHub
git push origin main

# 2. Откройте Actions
# https://github.com/YOUR_USERNAME/TaxionBack/actions

# 3. Дождитесь завершения workflow (5-10 минут)

# 4. Проверьте пакеты
# https://github.com/YOUR_USERNAME?tab=packages

# 5. Или pull'ните образ
docker pull ghcr.io/YOUR_USERNAME/tachyon-user:latest
```

---

## 📊 Статистика и аналитика

GitHub показывает для каждого образа:

**Downloads:**
- Количество pull'ов образа
- По тегам

**Storage:**
- Размер всех версий
- Использование квоты (500MB для free, 2GB для Pro)

**Versions:**
- История всех тегов
- Возможность удалить старые версии

---

## 🧹 Очистка старых образов

### Ручное удаление через веб

1. Откройте пакет
2. Выберите тег (версию)
3. Нажмите "Delete"

### Через API

```bash
# Получить список версий
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://api.github.com/users/YOUR_USERNAME/packages/container/tachyon-user/versions

# Удалить конкретную версию
curl -X DELETE \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://api.github.com/users/YOUR_USERNAME/packages/container/tachyon-user/versions/VERSION_ID
```

### Автоматическая очистка (через Actions)

Можно настроить автоматическое удаление старых версий:

```yaml
# .github/workflows/cleanup-old-images.yml
name: Cleanup old images

on:
  schedule:
    - cron: '0 0 * * 0'  # Каждое воскресенье в полночь

jobs:
  cleanup:
    runs-on: ubuntu-latest
    steps:
      - name: Delete old images
        uses: actions/delete-package-versions@v4
        with:
          package-name: 'tachyon-user'
          package-type: 'container'
          min-versions-to-keep: 10
          delete-only-untagged-versions: 'true'
```

---

## ✅ Проверка что образы доступны

```bash
# 1. Логин
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin

# 2. Pull образа
docker pull ghcr.io/YOUR_USERNAME/tachyon-user:latest

# 3. Если успешно - образ доступен!
# Вывод:
# latest: Pulling from your-username/tachyon-user
# abc123: Pull complete
# def456: Pull complete
# Status: Downloaded newer image for ghcr.io/your-username/tachyon-user:latest
```

---

## 📚 Полезные ссылки

- **Ваши пакеты:** `https://github.com/YOUR_USERNAME?tab=packages`
- **GitHub Actions:** `https://github.com/YOUR_USERNAME/TaxionBack/actions`
- **GHCR документация:** https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
- **API Reference:** https://docs.github.com/en/rest/packages

---

## 🆘 Troubleshooting

### Не вижу пакеты на странице профиля

**Причина:** Пакеты приватные и видны только вам

**Решение:** Залогиньтесь на GitHub

---

### Образ не pull'ится

**Причина:** Нет доступа или образ не существует

**Решение:**
```bash
# Проверьте авторизацию
docker login ghcr.io

# Проверьте что образ существует через веб
# https://github.com/YOUR_USERNAME?tab=packages
```

---

### GitHub Actions не публикует образы

**Причина:** Нет прав или неправильная конфигурация

**Решение:**
1. Проверьте что workflow запущен
2. Проверьте логи в Actions
3. Проверьте что `GITHUB_TOKEN` имеет права `write:packages`

---

**Быстрый доступ к вашим образам:** `https://github.com/YOUR_USERNAME?tab=packages`
