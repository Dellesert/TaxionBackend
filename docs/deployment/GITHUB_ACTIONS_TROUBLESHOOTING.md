# GitHub Actions Troubleshooting

## Проблема: Ошибка attestation (Unable to get ACTIONS_ID_TOKEN_REQUEST_URL)

### Описание ошибки

```
Run actions/attest-build-provenance@v1
Error: Failed to get ID token: Error message: Unable to get ACTIONS_ID_TOKEN_REQUEST_URL env variable
```

### Причина

Attestation (подпись образов) - это новая функция безопасности GitHub для проверки подлинности образов. Для её работы требуются дополнительные permissions в workflow.

---

## Решение 1: Добавить необходимые права ✅ (рекомендуется)

Обновлён файл [.github/workflows/docker-publish.yml](../../.github/workflows/docker-publish.yml)

### Что добавлено:

```yaml
permissions:
  contents: read
  packages: write
  id-token: write      # ← Добавлено для attestation
  attestations: write  # ← Добавлено для attestation
```

И добавлен `id` для шага сборки:

```yaml
- name: Build and push Docker image
  id: build  # ← Добавлено для ссылки в attestation
  uses: docker/build-push-action@v5
```

### Преимущества:
- ✅ Повышенная безопасность
- ✅ Подпись образов для проверки подлинности
- ✅ Соответствие best practices

### Недостатки:
- ⚠️ Требует дополнительных прав
- ⚠️ Может не работать в некоторых организациях

---

## Решение 2: Использовать упрощённый workflow (альтернатива)

Создан альтернативный файл [.github/workflows/docker-publish-simple.yml](../../.github/workflows/docker-publish-simple.yml)

### Что изменено:

Убран шаг attestation:

```yaml
# Убран этот шаг:
# - name: Generate artifact attestation
#   uses: actions/attest-build-provenance@v1
```

### Преимущества:
- ✅ Проще и надёжнее
- ✅ Не требует дополнительных прав
- ✅ Работает везде

### Недостатки:
- ⚠️ Нет подписи образов

---

## Решение 3: Полностью отключить attestation

Если хотите использовать основной workflow без attestation:

### Вариант A: Закомментировать в существующем файле

Откройте [.github/workflows/docker-publish.yml](../../.github/workflows/docker-publish.yml)

```yaml
# Закомментируйте эти строки:
# - name: Generate artifact attestation
#   if: github.event_name != 'pull_request'
#   uses: actions/attest-build-provenance@v1
#   with:
#     subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}-${{ matrix.service }}
#     subject-digest: ${{ steps.build.outputs.digest }}
#     push-to-registry: true
```

### Вариант B: Переименовать файлы

```bash
# Отключить основной workflow (с attestation)
mv .github/workflows/docker-publish.yml .github/workflows/docker-publish.yml.disabled

# Использовать простой workflow
mv .github/workflows/docker-publish-simple.yml .github/workflows/docker-publish.yml
```

---

## Какой вариант выбрать?

### Для большинства проектов: Решение 1 ✅

Используйте обновлённый `docker-publish.yml` с правами для attestation.

**Когда:**
- Хотите максимальную безопасность
- У вас есть права на настройку permissions
- Публичный или важный проект

**Как проверить что работает:**
```bash
git add .github/workflows/docker-publish.yml
git commit -m "Fix attestation permissions"
git push origin main
```

Откройте Actions и проверьте что сборка прошла успешно.

---

### Для простоты: Решение 2 или 3 ⚡

Используйте упрощённый workflow без attestation.

**Когда:**
- Приватный проект
- Не нужна дополнительная безопасность
- Хотите избежать сложностей

**Как переключиться:**
```bash
# Вариант 1: Использовать простой workflow
git mv .github/workflows/docker-publish.yml .github/workflows/docker-publish-with-attestation.yml
git mv .github/workflows/docker-publish-simple.yml .github/workflows/docker-publish.yml
git commit -m "Use simple workflow without attestation"
git push origin main

# Вариант 2: Закомментировать attestation в основном файле
# Просто закомментируйте шаг "Generate artifact attestation"
```

---

## Другие частые ошибки GitHub Actions

### Ошибка: "Resource not accessible by integration"

**Причина:** Недостаточно прав у GITHUB_TOKEN

**Решение:**
1. Откройте: Settings → Actions → General
2. В секции "Workflow permissions" выберите:
   - ✅ "Read and write permissions"

---

### Ошибка: "Docker build failed"

**Причина:** Ошибка в Dockerfile или коде

**Решение:**
1. Откройте логи в Actions
2. Найдите конкретную ошибку
3. Исправьте и push снова

---

### Ошибка: "Rate limit exceeded"

**Причина:** Превышен лимит запросов к GitHub API

**Решение:**
Подождите 1 час или используйте cache:

```yaml
- name: Build and push
  uses: docker/build-push-action@v5
  with:
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

---

### Ошибка: "No space left on device"

**Причина:** Закончилось место на runner

**Решение:**
Добавьте очистку перед сборкой:

```yaml
- name: Free disk space
  run: |
    docker system prune -af
    df -h
```

---

## Проверка что workflow работает

### 1. Через веб-интерфейс

```
https://github.com/YOUR_USERNAME/TaxionBack/actions
```

Должны увидеть:
- ✅ Зелёная галочка - успешно
- 🟡 Жёлтый кружок - в процессе
- ❌ Красный крестик - ошибка

---

### 2. Проверка образов

После успешной сборки:

```
https://github.com/YOUR_USERNAME?tab=packages
```

Должны увидеть все образы:
- tachyon-user
- tachyon-chat
- tachyon-task
- и т.д.

---

### 3. Pull образа

```bash
docker pull ghcr.io/YOUR_USERNAME/tachyon-user:latest
```

Если успешно - всё работает! ✅

---

## Дополнительная настройка

### Уведомления о сборках

Добавьте в конец workflow:

```yaml
- name: Notify on success
  if: success()
  run: echo "✅ Build successful!"

- name: Notify on failure
  if: failure()
  run: |
    echo "❌ Build failed!"
    echo "Check logs at: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}"
```

---

### Оптимизация времени сборки

Включите кеширование (уже есть в workflow):

```yaml
cache-from: type=gha
cache-to: type=gha,mode=max
```

Это уменьшит время сборки с 10 минут до 3-5 минут при повторных сборках.

---

## Мониторинг

### Просмотр логов

```
1. Откройте Actions
2. Кликните на запуск workflow
3. Выберите конкретную job (service)
4. Увидите детальные логи каждого шага
```

### Email уведомления

GitHub автоматически отправляет email при:
- ❌ Ошибке сборки
- ✅ Первой успешной сборке после ошибки

Настройка: Settings → Notifications → Actions

---

## FAQ

**Q: Нужен ли attestation для работы?**
A: Нет, это опциональная функция безопасности.

**Q: Какой workflow лучше?**
A: Для приватных проектов - без attestation (проще). Для публичных - с attestation (безопаснее).

**Q: Можно ли отключить сборку для конкретных веток?**
A: Да, измените условие `on: push: branches:`

**Q: Сколько занимает сборка?**
A: Первая - 10-15 минут. С кешем - 3-5 минут.

**Q: Есть ли лимиты?**
A: Да, 2000 минут/месяц для приватных репозиториев (бесплатный аккаунт). Публичные - безлимитно.

---

## Итого

### Что сделано:

1. ✅ Исправлен основной workflow с attestation
   - Добавлены permissions: `id-token: write`, `attestations: write`
   - Добавлен `id: build` для шага сборки

2. ✅ Создан альтернативный простой workflow
   - Без attestation
   - Проще и надёжнее
   - Файл: `docker-publish-simple.yml`

3. ✅ Документация
   - Объяснение проблемы
   - Несколько решений
   - Troubleshooting

### Рекомендация:

Используйте **исправленный основной workflow** ([docker-publish.yml](../../.github/workflows/docker-publish.yml)).

Если возникнут проблемы - переключитесь на упрощённый вариант.

---

**Проверка:** [GitHub Actions](https://github.com/YOUR_USERNAME/TaxionBack/actions)

**Образы:** [GitHub Packages](https://github.com/YOUR_USERNAME?tab=packages)
