# CI/CD Шпаргалка - Автоматическая сборка образов

## 🤖 Когда собираются образы автоматически

### 1. Push в main ✅ АВТОМАТИЧЕСКИ
```bash
git add .
git commit -m "Add new feature"
git push origin main
```
**Результат:**
- Образы собираются автоматически через 1-2 минуты
- Теги: `latest`, `main`, `main-abc123def`
- Время сборки: ~5-10 минут

---

### 2. Push в develop ✅ АВТОМАТИЧЕСКИ
```bash
git push origin develop
```
**Результат:**
- Образы собираются автоматически
- Тег: `develop`
- Для staging окружения

---

### 3. Создание тега версии ✅ АВТОМАТИЧЕСКИ (рекомендуется)
```bash
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0
```
**Результат:**
- Образы собираются автоматически
- Теги: `v1.0.0`, `v1.0`, `v1`, `latest` (если из main)
- Для production деплоя

---

### 4. Pull Request в main ✅ ТОЛЬКО СБОРКА (не публикуется)
```bash
# Создайте PR на GitHub
```
**Результат:**
- Образы собираются для проверки
- НЕ публикуются в GHCR
- Только проверка что всё компилируется

---

### 5. Ручной запуск ✅ ВРУЧНУЮ
```
1. Откройте: https://github.com/YOUR_USERNAME/TaxionBack/actions
2. Выберите "Build and Push Docker Images"
3. Нажмите "Run workflow"
4. Укажите кастомную версию (опционально)
```
**Результат:**
- Образы собираются с указанной версией
- Для особых случаев

---

## 📦 Где посмотреть образы

### Быстрый доступ
```
https://github.com/YOUR_USERNAME?tab=packages
```

### Через репозиторий
```
1. Откройте: https://github.com/YOUR_USERNAME/TaxionBack
2. Справа увидите секцию "Packages"
3. Кликните на пакет
```

### Через командную строку
```bash
# Логин
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin

# Pull образа
docker pull ghcr.io/YOUR_USERNAME/tachyon-user:latest

# Список локальных образов
docker images | grep tachyon
```

---

## 🚀 Типичный workflow

### Development → Production

```bash
# 1. Разработка в feature ветке
git checkout -b feature/new-feature
# ... код ...
git push origin feature/new-feature

# 2. PR в develop для staging
# Создайте PR: feature/new-feature → develop
# После merge образы соберутся автоматически с тегом develop

# 3. Тестирование на staging
docker compose -f docker-compose.staging.yml pull
docker compose -f docker-compose.staging.yml up -d

# 4. PR в main для production
# Создайте PR: develop → main
# После merge образы соберутся с тегом main

# 5. Релиз версии
git checkout main
git pull
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
# → Образы соберутся с v1.0.0

# 6. Деплой на production
# Обновите docker-compose.prod.yml:
sed -i 's/:latest/:v1.0.0/g' docker-compose.prod.yml
./scripts/deployment/deploy.sh deploy
```

---

## 🔄 Процесс автоматической сборки

```
Push в GitHub
      ↓
GitHub Actions запускается (1-2 мин)
      ↓
Сборка образов параллельно (5-10 мин)
  ├─ user-service
  ├─ chat-service
  ├─ task-service
  ├─ file-service
  ├─ calendar-service
  ├─ poll-service
  ├─ notification-service
  ├─ analytics-service
  ├─ gateway
  └─ backup-service
      ↓
Push в GHCR (1-2 мин)
      ↓
Образы доступны для pull
```

**Общее время:** 7-14 минут

---

## 📋 Проверка статуса сборки

### Через веб
```
1. Откройте: https://github.com/YOUR_USERNAME/TaxionBack/actions
2. Найдите последний запуск "Build and Push Docker Images"
3. Статусы:
   - 🟡 В процессе (желтый кружок)
   - ✅ Успешно (зеленая галочка)
   - ❌ Ошибка (красный крестик)
```

### Через email
GitHub отправит email при:
- ❌ Ошибке сборки
- ✅ Успешной сборке (если настроено)

### Через CLI
```bash
# Установка gh CLI
brew install gh  # macOS
# или
sudo apt install gh  # Linux

# Проверка последнего запуска
gh run list --workflow="Build and Push Docker Images"

# Детали конкретного запуска
gh run view RUN_ID
```

---

## 🏷️ Теги образов

### После push в main
```
ghcr.io/username/tachyon-user:latest
ghcr.io/username/tachyon-user:main
ghcr.io/username/tachyon-user:main-abc123def
```

### После push тега v1.2.3
```
ghcr.io/username/tachyon-user:v1.2.3
ghcr.io/username/tachyon-user:v1.2
ghcr.io/username/tachyon-user:v1
ghcr.io/username/tachyon-user:latest  (если из main)
```

### После push в develop
```
ghcr.io/username/tachyon-user:develop
```

---

## ⚡ Быстрые команды

### Создать релиз и задеплоить
```bash
# 1. Создать тег
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 2. Дождаться сборки (проверить в Actions)
# https://github.com/YOUR_USERNAME/TaxionBack/actions

# 3. На сервере обновить версии
sed -i 's/:latest/:v1.0.0/g' docker-compose.prod.yml

# 4. Деплой
./scripts/deployment/deploy.sh deploy
```

### Просмотреть доступные образы
```bash
# Веб
open https://github.com/YOUR_USERNAME?tab=packages

# CLI
docker search ghcr.io/YOUR_USERNAME/tachyon
```

### Откат на предыдущую версию
```bash
# В docker-compose.prod.yml
sed -i 's/:v1.1.0/:v1.0.0/g' docker-compose.prod.yml
./scripts/deployment/deploy.sh deploy
```

---

## 🛠️ Настройка уведомлений

### Email уведомления (GitHub)
```
1. GitHub Settings → Notifications
2. Включите "Actions"
3. Выберите когда получать:
   - On failure (при ошибках)
   - Always (всегда)
```

### Slack/Discord уведомления
Добавьте в `.github/workflows/docker-publish.yml`:

```yaml
- name: Notify Slack
  if: success()
  uses: 8398a7/action-slack@v3
  with:
    status: ${{ job.status }}
    webhook_url: ${{ secrets.SLACK_WEBHOOK }}
```

---

## 📊 Мониторинг сборок

### Время сборки
- **Быстро:** 5-7 минут (с кешем)
- **Обычно:** 7-10 минут
- **Долго:** 10-15 минут (первая сборка или изменения зависимостей)

### Размер образов
- **User/Chat/Task:** ~150-200 MB
- **Gateway:** ~150 MB
- **File Service:** ~200-250 MB
- **Notification:** ~180 MB
- **Все вместе:** ~1.5-2 GB

### Квоты GHCR (бесплатный аккаунт)
- **Storage:** 500 MB (хранилище)
- **Data transfer:** 1 GB/месяц (скачивание)

**Совет:** Удаляйте старые версии!

---

## 🆘 Troubleshooting

### Сборка не запускается

**Причина:** Workflow не настроен или отключен

**Решение:**
```bash
# Проверьте что файл существует
ls .github/workflows/docker-publish.yml

# Проверьте на GitHub
# Settings → Actions → General → Allow actions
```

---

### Сборка падает с ошибкой

**Причина:** Ошибка в коде или Dockerfile

**Решение:**
1. Откройте логи в Actions
2. Найдите красную строку с ошибкой
3. Исправьте код
4. Push снова - пересоберётся автоматически

---

### Образы не публикуются

**Причина:** Недостаточно прав у GITHUB_TOKEN

**Решение:**
Проверьте Settings → Actions → General → Workflow permissions:
- ✅ "Read and write permissions"

---

### Образы не pull'ятся на сервере

**Причина:** Не авторизованы в GHCR

**Решение:**
```bash
source ~/.github-credentials
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

---

## 📚 Дополнительная информация

- **Детальное руководство:** [VIEWING_IMAGES.md](VIEWING_IMAGES.md)
- **Настройка авторизации:** [../GITHUB_AUTH_SETUP.md](../GITHUB_AUTH_SETUP.md)
- **Быстрый деплой:** [../QUICK_DEPLOY.md](../QUICK_DEPLOY.md)

---

**Ваши образы:** `https://github.com/YOUR_USERNAME?tab=packages`

**GitHub Actions:** `https://github.com/YOUR_USERNAME/TaxionBack/actions`
