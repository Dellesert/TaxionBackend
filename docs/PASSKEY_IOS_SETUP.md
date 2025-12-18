# Passkey Setup для iOS

Это руководство описывает настройку Passkey (WebAuthn) для iOS приложения с поддержкой нескольких доменов.

## Оглавление

- [Архитектура решения](#архитектура-решения)
- [Настройка Backend](#настройка-backend)
- [Настройка iOS приложения](#настройка-ios-приложения)
- [Настройка Apple Developer Portal](#настройка-apple-developer-portal)
- [Troubleshooting](#troubleshooting)

---

## Архитектура решения

### Проблема

iOS требует строгого соответствия между:
- **RP_ID** (Relying Party ID) в WebAuthn
- **Доменом** в `apple-app-site-association`
- **Associated Domains** в приложении

Когда у вас несколько доменов (например, `dash.fusioninsight.cloud` для админки и `taxion.fusioninsight.cloud` для API), нужна поддержка динамического выбора RP_ID.

### Решение

**Multi-origin WebAuthn Service** автоматически выбирает правильный RP_ID на основе HTTP заголовка `Origin`:

```
https://dash.fusioninsight.cloud → RP_ID: dash.fusioninsight.cloud
https://taxion.fusioninsight.cloud → RP_ID: taxion.fusioninsight.cloud
```

---

## Настройка Backend

### 1. Переменные окружения

**`.env` на продакшене:**

```bash
WEBAUTHN_RP_DISPLAY_NAME=Tachyon Messenger
WEBAUTHN_RP_ORIGIN=https://dash.fusioninsight.cloud,https://taxion.fusioninsight.cloud
```

**Важно:**
- ❌ **НЕ указывайте** `WEBAUTHN_RP_ID` - он вычисляется автоматически
- ✅ Перечислите все origins через запятую
- ✅ Используйте полные URL с протоколом

### 2. WebAuthn Service

Файл: `services/user/usecase/webauthn_service.go`

**Функционал:**
- Автоматически извлекает RP_ID из Origin
- Создает отдельный WebAuthn инстанс для каждого уникального домена
- Выбирает правильный инстанс на основе HTTP заголовка

**Пример логов при старте:**
```
INFO Configured WebAuthn RP_ID mapping origin=https://dash.fusioninsight.cloud rp_id=dash.fusioninsight.cloud
INFO Configured WebAuthn RP_ID mapping origin=https://taxion.fusioninsight.cloud rp_id=taxion.fusioninsight.cloud
INFO Created WebAuthn instance rp_id=dash.fusioninsight.cloud
INFO Created WebAuthn instance rp_id=taxion.fusioninsight.cloud
```

### 3. Handlers

Все Passkey handlers извлекают Origin из HTTP заголовков:

```go
// Get origin from request header
origin := c.GetHeader("Origin")
if origin == "" {
    // Fallback to Referer header if Origin is not set
    origin = c.GetHeader("Referer")
}
```

### 4. Well-known файлы

**Важно:** Файл `apple-app-site-association` должен быть доступен на **всех** доменах:

```bash
curl https://dash.fusioninsight.cloud/.well-known/apple-app-site-association
curl https://taxion.fusioninsight.cloud/.well-known/apple-app-site-association
```

**Оба должны возвращать:**
- HTTP Status: **200 OK**
- Content-Type: **application/json** (или без заголовка)
- JSON с вашими Bundle IDs

**Пример содержимого:**
```json
{
  "webcredentials": {
    "apps": [
      "QNVQ55232N.com.dellesert.tachyon-messenger",
      "QNVQ55232N.com.dellesert.tachyon-messenger.release"
    ]
  },
  "applinks": {
    "apps": [],
    "details": [
      {
        "appID": "QNVQ55232N.com.dellesert.tachyon-messenger",
        "paths": ["/.well-known/*"]
      }
    ]
  }
}
```

**Настройка Gateway:**

Файл: `services/gateway/main.go`

```go
// .well-known endpoints for iOS/Android Universal Links and Passkeys
router.GET("/.well-known/apple-app-site-association", serveAppleAppSiteAssociation)

func serveAppleAppSiteAssociation(c *gin.Context) {
    c.Header("Content-Type", "application/json")
    c.File("./.well-known/apple-app-site-association")
}
```

**Dockerfile:**
```dockerfile
# Copy .well-known directory for iOS/Android Universal Links and Passkeys
COPY .well-known/ ./.well-known/
```

---

## Настройка iOS приложения

### 1. Entitlements файл

**Файл:** `ios/Dev/Dev.entitlements`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>aps-environment</key>
    <string>production</string>
    <key>com.apple.developer.associated-domains</key>
    <array>
      <string>webcredentials:taxion.fusioninsight.cloud</string>
    </array>
  </dict>
</plist>
```

**Важно:**
- ✅ Формат: `webcredentials:DOMAIN` (без https://)
- ✅ Укажите домен, который использует iOS приложение для API
- ❌ Не добавляйте порты или пути

### 2. Xcode Configuration

#### Способ 1: Через UI

1. Откройте `ios/Dev.xcworkspace` в Xcode
2. Выберите target **Dev**
3. Перейдите на вкладку **Signing & Capabilities**
4. Нажмите **+ Capability** → **Associated Domains**
5. Добавьте: `webcredentials:taxion.fusioninsight.cloud`

#### Способ 2: Проверка в project.pbxproj

Файл `ios/Dev.xcodeproj/project.pbxproj` должен содержать:

```
CODE_SIGN_ENTITLEMENTS = Dev/Dev.entitlements;
```

### 3. React Native Passkey код

**Файл:** `src/shared/utils/passkeyUtils.ts`

```typescript
export async function registerPasskey(challenge: string, options: any): Promise<any> {
  if (Platform.OS === 'web') {
    // Веб: используем WebAuthn API
    // ...
  } else {
    // iOS/Android: используем react-native-passkey
    const result = await Passkey.create(options.publicKey);
    return result;
  }
}
```

**Важно:**
- ✅ На iOS используется `react-native-passkey` библиотека
- ✅ Передается весь объект `options.publicKey` от сервера
- ✅ RP_ID приходит от бэкенда и автоматически соответствует домену

---

## Настройка Apple Developer Portal

### 1. App ID Configuration

1. Откройте [Apple Developer Portal](https://developer.apple.com/account/)
2. **Certificates, Identifiers & Profiles** → **Identifiers**
3. Найдите: `com.dellesert.tachyon-messenger`
4. Нажмите **Edit**
5. Найдите **Associated Domains**
6. ✅ Включите checkbox
7. Нажмите **Save**

### 2. Provisioning Profile

После включения Associated Domains:

1. **Certificates, Identifiers & Profiles** → **Profiles**
2. Найдите профиль для вашего приложения
3. Нажмите на профиль → **Edit**
4. Нажмите **Generate** / **Regenerate**
5. **Download** обновленный `.mobileprovision` файл
6. Установите: двойной клик на файл

### 3. Проверка

**Убедитесь:**
- ✅ Associated Domains включен в App ID
- ✅ Provisioning Profile regenerated после включения
- ✅ Профиль установлен в Xcode

---

## Build и Deployment

### Clean Build (обязательно!)

После изменения entitlements нужен clean build:

```bash
cd ios
xcodebuild clean -workspace Dev.xcworkspace -scheme Dev
xcodebuild -workspace Dev.xcworkspace -scheme Dev
```

Или в Xcode:
```
Product → Clean Build Folder (Shift + Cmd + K)
Product → Build (Cmd + B)
```

### Запуск на устройстве

**Важно:** Passkey **НЕ работает на симуляторе** для production доменов!

```bash
# Через Expo
npx expo run:ios --device

# Или через Xcode
# Выберите реальное устройство и нажмите Run
```

### Деплой бэкенда

```bash
cd /path/to/TaxionBack

# Build и restart user-service
docker-compose -f docker-compose.prod.yml build user-service
docker-compose -f docker-compose.prod.yml up -d user-service

# Проверка логов
docker-compose -f docker-compose.prod.yml logs -f user-service | grep -i "origin\|rp_id"
```

**Ожидаемые логи:**
```
INFO Configured WebAuthn RP_ID mapping origin=https://taxion.fusioninsight.cloud rp_id=taxion.fusioninsight.cloud
INFO BeginRegistration with origin origin=https://taxion.fusioninsight.cloud
```

---

## Troubleshooting

### ❌ Ошибка: "The request failed. No Credentials were returned"

**Возможные причины:**

#### 1. Associated Domains не настроен

**Проверка:**
```bash
# Проверьте entitlements в собранном приложении
codesign -d --entitlements - /path/to/Dev.app
```

**Ожидается:**
```xml
<key>com.apple.developer.associated-domains</key>
<array>
  <string>webcredentials:taxion.fusioninsight.cloud</string>
</array>
```

**Решение:**
- Убедитесь, что `Dev.entitlements` содержит Associated Domains
- Выполните Clean Build
- Пересоберите приложение

#### 2. Well-known файл недоступен

**Проверка:**
```bash
curl -I https://taxion.fusioninsight.cloud/.well-known/apple-app-site-association
```

**Ожидается:**
```
HTTP/1.1 200 OK
Content-Type: application/json
```

**Решение:**
- Убедитесь, что Gateway сервис задеплоен с `.well-known` файлами
- Проверьте nginx/reverse proxy конфигурацию
- Проверьте HTTPS сертификаты

#### 3. Неправильный RP_ID

**Проверка логов бэкенда:**
```bash
docker logs user-service 2>&1 | grep "BeginRegistration with origin"
```

**Ожидается:**
```
DEBUG BeginRegistration with origin origin=https://taxion.fusioninsight.cloud
```

**Если origin пустой или неправильный:**
- React Native может не отправлять Origin header
- Проверьте axios конфигурацию
- Добавьте header вручную в API запросах

#### 4. Приложение запущено на симуляторе

**Симптомы:**
- Passkey работает в браузере
- Не работает в iOS приложении на симуляторе

**Решение:**
- ✅ Используйте **реальное iOS устройство**
- ❌ Симулятор не поддерживает Passkey для production доменов

#### 5. Associated Domains не включен в Developer Portal

**Проверка:**
1. Apple Developer Portal → Identifiers → Your App ID
2. Проверьте, что **Associated Domains** отмечен галочкой

**Решение:**
- Включите Associated Domains в App ID
- Regenerate Provisioning Profile
- Download и установите новый профиль

---

## Логи для отладки

### iOS Console (Mac)

1. Подключите iOS устройство к Mac
2. Откройте **Console.app**
3. Выберите ваше устройство
4. Фильтры:
   - `swcd` - для Associated Domains
   - `authenticationservices` - для Passkey/WebAuthn

**Полезные логи:**
```
swcd: Successfully downloaded apple-app-site-association for taxion.fusioninsight.cloud
authenticationservices: Presenting passkey registration UI
```

**Ошибки:**
```
swcd: Failed to download apple-app-site-association (404)
authenticationservices: Domain not approved for webcredentials
```

### Backend логи

```bash
# Полные логи user-service
docker-compose -f docker-compose.prod.yml logs -f user-service

# Только Passkey related
docker-compose -f docker-compose.prod.yml logs -f user-service | grep -i "passkey\|webauthn\|origin"
```

---

## Проверочный чеклист

### Backend

- [ ] `WEBAUTHN_RP_ORIGIN` содержит все домены через запятую
- [ ] `WEBAUTHN_RP_ID` **НЕ** указан (автоматически)
- [ ] Well-known файл доступен: `curl https://taxion.fusioninsight.cloud/.well-known/apple-app-site-association`
- [ ] Gateway задеплоен с `.well-known` директорией
- [ ] User-service перезапущен с новой конфигурацией
- [ ] Логи показывают правильный RP_ID mapping

### iOS

- [ ] `Dev.entitlements` содержит `com.apple.developer.associated-domains`
- [ ] Associated Domains включен в Apple Developer Portal
- [ ] Provisioning Profile regenerated
- [ ] Профиль установлен в Xcode
- [ ] Clean Build выполнен
- [ ] Приложение установлено на **реальное устройство**
- [ ] Bundle ID совпадает с `apple-app-site-association`

### Тестирование

- [ ] Откройте приложение на iOS устройстве
- [ ] Попробуйте зарегистрировать Passkey
- [ ] iOS показывает промпт: "Would Like to Use ... to Sign In"
- [ ] FaceID/TouchID промпт появляется
- [ ] Passkey успешно создается

---

## Дополнительные ресурсы

- [Apple - Supporting associated domains](https://developer.apple.com/documentation/xcode/supporting-associated-domains)
- [Apple - Supporting passkeys](https://developer.apple.com/documentation/authenticationservices/public-private_key_authentication/supporting_passkeys)
- [WebAuthn Guide](https://webauthn.guide/)
- [react-native-passkey](https://github.com/f-23/react-native-passkey)

---

## История изменений

### 2024-12-18
- Добавлена поддержка multi-origin WebAuthn
- Динамический выбор RP_ID на основе Origin header
- Обновлены handlers для извлечения Origin
- Добавлен Associated Domains в iOS entitlements

---

**Автор:** Claude Code
**Последнее обновление:** 18 декабря 2024
