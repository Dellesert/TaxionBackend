# Настройка Deep Linking для мобильного приложения (Инвайты и Сброс пароля)

## Обзор решения

Реализован гибридный подход для **инвайтов** и **сброса пароля**, который работает надежно во всех email-клиентах:

1. **Email содержит обычную HTTPS ссылку** на backend (`https://yourdomain.com/reset-password/:token`)
2. **Backend возвращает HTML страницу** с JavaScript для определения платформы
3. **Страница пытается открыть приложение** через Universal Links (iOS) / App Links (Android)
4. **Fallback**: Если приложение не установлено, показываются инструкции и ссылки на store

## Важно: Настройка BACKEND_URL

### Для локальной разработки

В файле `.env` добавьте:
```bash
# URL для email-ссылок (должен указывать на GATEWAY, не на user-service напрямую)
BACKEND_URL=http://localhost:8080

# Альтернативно можно использовать:
# BACKEND_URL=http://host.docker.internal:8080  # для доступа к хосту из Docker
```

**ВАЖНО**: Ссылки в письмах должны вести через API Gateway (порт 8080), а не напрямую на user-service (порт 8081).

### Для production

В `.env.production` или переменных окружения:
```bash
BACKEND_URL=https://yourdomain.com
```

Gateway автоматически проксирует запросы к нужным сервисам.

## Что было реализовано

### Backend (Go)

#### Password Reset

✅ **Handler для редиректа** - `services/user/handlers/password_reset_handler.go`:
- `PasswordResetRedirect()` - HTML страница с определением платформы
- Валидация токена перед показом страницы
- Красивый UI с анимацией загрузки
- Автоматическое копирование токена в буфер обмена

✅ **Маршрут** - добавлен в `services/user/main.go`:
```go
router.GET("/reset-password/:token", passwordResetHandler.PasswordResetRedirect)
```

✅ **Обновленная генерация ссылок** - `services/user/usecase/password_reset_usecase.go`:
- Теперь использует `BACKEND_URL` вместо `FRONTEND_URL`
- Fallback на `USER_SERVICE_URL` или `http://localhost:8081`

✅ **Обновленный email шаблон** - `shared/email/email.go`:
- Убраны упоминания о веб-версии
- Добавлены инструкции для мобильного приложения
- Улучшенный UX для пользователей

#### Invitations

✅ **Handler для редиректа** - `services/user/handlers/invitation_handler.go:545-887`
- `InvitationRedirect()` - HTML страница с определением платформы
- Валидация приглашения перед показом страницы
- Персонализация (показ имени пользователя)
- Красивый UI с брендовыми цветами (красный)

✅ **Маршрут** - добавлен в `services/user/main.go:323`
```go
router.GET("/invite/:token", invitationHandler.InvitationRedirect)
```

✅ **Обновленная генерация ссылок** - `services/user/usecase/invitation_usecase.go:488-500`
- Теперь использует `BACKEND_URL` вместо `FRONTEND_URL`
- Ссылки ведут на backend: `https://yourdomain.com/invite/TOKEN`

✅ **Обновлен email шаблон** - `shared/email/invitation_template.go`
- Добавлена кнопка "Принять приглашение" с backend URL
- Добавлены инструкции "Как это работает"
- Сохранены подробные шаги для ручной активации

## Настройка React Native приложения

### 1. Установите необходимые пакеты

```bash
npm install react-native-inappbrowser-reborn
# или
yarn add react-native-inappbrowser-reborn
```

### 2. Настройте Deep Linking

#### iOS - Universal Links

**Файл: `ios/YourApp/YourApp.entitlements`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.developer.associated-domains</key>
    <array>
        <string>applinks:yourdomain.com</string>
    </array>
</dict>
</plist>
```

**Файл на сервере: `https://yourdomain.com/.well-known/apple-app-site-association`**

```json
{
  "applinks": {
    "apps": [],
    "details": [
      {
        "appID": "TEAM_ID.com.yourcompany.tachyon",
        "paths": ["/reset-password/*"]
      }
    ]
  }
}
```

#### Android - App Links

**Файл: `android/app/src/main/AndroidManifest.xml`**

```xml
<activity
    android:name=".MainActivity"
    android:launchMode="singleTask">

    <!-- Deep linking -->
    <intent-filter>
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="tachyon" />
    </intent-filter>

    <!-- App Links (verified links) -->
    <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data
            android:scheme="https"
            android:host="yourdomain.com"
            android:pathPrefix="/reset-password" />
    </intent-filter>
</activity>
```

**Файл на сервере: `https://yourdomain.com/.well-known/assetlinks.json`**

```json
[{
  "relation": ["delegate_permission/common.handle_all_urls"],
  "target": {
    "namespace": "android_app",
    "package_name": "com.yourcompany.tachyon",
    "sha256_cert_fingerprints": [
      "YOUR_APP_SHA256_FINGERPRINT"
    ]
  }
}]
```

### 3. Обработка deep links в React Native

**Файл: `App.tsx` или главный компонент**

```typescript
import { useEffect } from 'react';
import { Linking } from 'react-native';
import { useNavigation } from '@react-navigation/native';

function App() {
  const navigation = useNavigation();

  useEffect(() => {
    // Handle initial URL (app was closed)
    Linking.getInitialURL().then((url) => {
      if (url) {
        handleDeepLink(url);
      }
    });

    // Handle URL when app is already open
    const subscription = Linking.addEventListener('url', ({ url }) => {
      handleDeepLink(url);
    });

    return () => {
      subscription.remove();
    };
  }, []);

  const handleDeepLink = (url: string) => {
    console.log('Received deep link:', url);

    // Parse URL - support both formats:
    // 1. tachyon://reset-password/TOKEN or tachyon://invite/TOKEN
    // 2. https://yourdomain.com/reset-password/TOKEN or https://yourdomain.com/invite/TOKEN

    // Check for password reset
    const resetMatch = url.match(/reset-password\/([^/?]+)/);
    if (resetMatch && resetMatch[1]) {
      const token = resetMatch[1];
      console.log('Reset password token:', token);
      navigation.navigate('ResetPassword', { token });
      return;
    }

    // Check for invitation
    const inviteMatch = url.match(/invite\/([^/?]+)/);
    if (inviteMatch && inviteMatch[1]) {
      const token = inviteMatch[1];
      console.log('Invitation token:', token);
      navigation.navigate('AcceptInvitation', { token });
      return;
    }

    console.warn('Unknown deep link format:', url);
  };

  return (
    // Your app components
  );
}
```

### 4. Экран сброса пароля

**Файл: `screens/ResetPasswordScreen.tsx`**

```typescript
import React, { useState, useEffect } from 'react';
import { View, Text, TextInput, Button, Alert } from 'react-native';
import axios from 'axios';

export default function ResetPasswordScreen({ route }) {
  const { token } = route.params;
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isValid, setIsValid] = useState(false);

  useEffect(() => {
    // Validate token
    validateToken();
  }, [token]);

  const validateToken = async () => {
    try {
      const response = await axios.get(
        `https://yourdomain.com/api/v1/password-resets/validate/${token}`
      );
      setIsValid(response.data.valid);
    } catch (error) {
      Alert.alert('Ошибка', 'Ссылка недействительна или истекла');
    }
  };

  const handleResetPassword = async () => {
    if (password !== confirmPassword) {
      Alert.alert('Ошибка', 'Пароли не совпадают');
      return;
    }

    try {
      await axios.post(
        `https://yourdomain.com/api/v1/password-resets/reset/${token}`,
        {
          password,
          confirm_password: confirmPassword,
        }
      );

      Alert.alert('Успешно', 'Пароль успешно изменен', [
        { text: 'OK', onPress: () => navigation.navigate('Login') }
      ]);
    } catch (error) {
      Alert.alert('Ошибка', error.response?.data?.error || 'Не удалось сбросить пароль');
    }
  };

  if (!isValid) {
    return (
      <View>
        <Text>Проверка ссылки...</Text>
      </View>
    );
  }

  return (
    <View style={{ padding: 20 }}>
      <Text style={{ fontSize: 24, marginBottom: 20 }}>Новый пароль</Text>

      <TextInput
        placeholder="Новый пароль"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
        style={{ borderWidth: 1, padding: 10, marginBottom: 10 }}
      />

      <TextInput
        placeholder="Подтвердите пароль"
        secureTextEntry
        value={confirmPassword}
        onChangeText={setConfirmPassword}
        style={{ borderWidth: 1, padding: 10, marginBottom: 20 }}
      />

      <Button title="Сбросить пароль" onPress={handleResetPassword} />
    </View>
  );
}
```

### 5. Экран принятия приглашения

**Файл: `screens/AcceptInvitationScreen.tsx`**

```typescript
import React, { useState, useEffect } from 'react';
import { View, Text, TextInput, Button, Alert, ScrollView } from 'react-native';
import axios from 'axios';

export default function AcceptInvitationScreen({ route, navigation }) {
  const { token } = route.params;
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isValid, setIsValid] = useState(false);
  const [invitationData, setInvitationData] = useState(null);

  useEffect(() => {
    // Validate invitation token
    validateInvitation();
  }, [token]);

  const validateInvitation = async () => {
    try {
      const response = await axios.get(
        `https://yourdomain.com/api/v1/invitations/validate/${token}`
      );
      setIsValid(response.data.valid);
      setInvitationData(response.data);
    } catch (error) {
      Alert.alert('Ошибка', 'Приглашение недействительно или истекло');
    }
  };

  const handleAcceptInvitation = async () => {
    if (password !== confirmPassword) {
      Alert.alert('Ошибка', 'Пароли не совпадают');
      return;
    }

    try {
      const response = await axios.post(
        `https://yourdomain.com/api/v1/invitations/accept/${token}`,
        {
          password,
          confirm_password: confirmPassword,
        }
      );

      // После успешного принятия приглашения автоматически входим
      const { token: authToken, user } = response.data;

      // Сохраните токен в AsyncStorage или secure storage
      // await AsyncStorage.setItem('auth_token', authToken);

      Alert.alert('Успешно', 'Добро пожаловать в Tachyon Messenger!', [
        { text: 'OK', onPress: () => navigation.replace('Home') }
      ]);
    } catch (error) {
      Alert.alert('Ошибка', error.response?.data?.error || 'Не удалось принять приглашение');
    }
  };

  if (!isValid || !invitationData) {
    return (
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        <Text>Проверка приглашения...</Text>
      </View>
    );
  }

  return (
    <ScrollView style={{ flex: 1 }}>
      <View style={{ padding: 20 }}>
        <Text style={{ fontSize: 24, marginBottom: 10, textAlign: 'center' }}>
          Добро пожаловать!
        </Text>

        <Text style={{ fontSize: 16, marginBottom: 20, textAlign: 'center', color: '#666' }}>
          {invitationData.name || invitationData.email}
        </Text>

        <View style={{ backgroundColor: '#f0f0f0', padding: 15, borderRadius: 8, marginBottom: 20 }}>
          <Text style={{ fontWeight: 'bold', marginBottom: 5 }}>Email:</Text>
          <Text>{invitationData.email}</Text>

          {invitationData.department && (
            <>
              <Text style={{ fontWeight: 'bold', marginTop: 10, marginBottom: 5 }}>Отдел:</Text>
              <Text>{invitationData.department}</Text>
            </>
          )}

          {invitationData.position && (
            <>
              <Text style={{ fontWeight: 'bold', marginTop: 10, marginBottom: 5 }}>Должность:</Text>
              <Text>{invitationData.position}</Text>
            </>
          )}
        </View>

        <Text style={{ fontSize: 18, marginBottom: 15, fontWeight: 'bold' }}>
          Создайте пароль
        </Text>

        <TextInput
          placeholder="Новый пароль (минимум 8 символов)"
          secureTextEntry
          value={password}
          onChangeText={setPassword}
          style={{
            borderWidth: 1,
            borderColor: '#ddd',
            padding: 12,
            marginBottom: 10,
            borderRadius: 8,
            fontSize: 16
          }}
        />

        <TextInput
          placeholder="Подтвердите пароль"
          secureTextEntry
          value={confirmPassword}
          onChangeText={setConfirmPassword}
          style={{
            borderWidth: 1,
            borderColor: '#ddd',
            padding: 12,
            marginBottom: 20,
            borderRadius: 8,
            fontSize: 16
          }}
        />

        <Button
          title="Принять приглашение"
          onPress={handleAcceptInvitation}
          color="#E94444"
        />
      </View>
    </ScrollView>
  );
}
```

## Переменные окружения

Добавьте в `.env` файл:

```bash
# URL вашего backend (для генерации ссылок в email)
BACKEND_URL=https://yourdomain.com
# Или, если используете отдельный URL для user service:
USER_SERVICE_URL=https://user-service.yourdomain.com

# Ссылки на магазины приложений (для fallback страниц)
APP_STORE_URL=https://apps.apple.com/app/tachyon-messenger/idYOUR_APP_ID
GOOGLE_PLAY_URL=https://play.google.com/store/apps/details?id=com.yourcompany.tachyon
```

**Важно:** Обновите ссылки на магазины когда опубликуете приложение!

## Тестирование

### 1. Локальное тестирование

```bash
# Запустите user service
cd services/user
go run .

# Откройте в браузере
http://localhost:8081/reset-password/test-token
```

### 2. Тестирование на устройстве

1. Создайте password reset через админку или API
2. Откройте email на мобильном устройстве
3. Нажмите на кнопку "Сбросить пароль"
4. Должна открыться страница, которая попытается открыть приложение
5. Если приложение установлено - откроется экран сброса пароля
6. Если нет - покажутся инструкции и ссылки на store

### 3. Проверка Universal/App Links

**iOS:**
```bash
# Проверьте файл на сервере
curl https://yourdomain.com/.well-known/apple-app-site-association
```

**Android:**
```bash
# Проверьте файл на сервере
curl https://yourdomain.com/.well-known/assetlinks.json

# Получите SHA256 fingerprint вашего приложения
cd android
./gradlew signingReport
```

## Что дальше?

1. **Настройте BACKEND_URL** в production окружении
2. **Загрузите файлы `.well-known`** на ваш домен
3. **Настройте Deep Linking** в React Native приложении
4. **Протестируйте** на реальных устройствах
5. **При необходимости обновите ссылки на App Store / Google Play** в HTML шаблоне

## Дополнительные возможности

### Аналитика

Добавьте трекинг переходов по ссылкам:

```go
// В handler
logger.WithFields(map[string]interface{}{
    "token":      token,
    "user_agent": c.GetHeader("User-Agent"),
    "platform":   detectPlatform(c.GetHeader("User-Agent")),
}).Info("Password reset page accessed")
```

### Кастомизация

Вы можете настроить внешний вид страницы редиректа в функции `getPasswordResetRedirectHTML()` в файле [services/user/handlers/password_reset_handler.go](services/user/handlers/password_reset_handler.go:266).

## Troubleshooting

**Приложение не открывается на iOS:**
- Проверьте, что домен добавлен в entitlements
- Убедитесь, что файл `.well-known/apple-app-site-association` доступен по HTTPS
- Переустановите приложение (iOS кэширует Universal Links)

**Приложение не открывается на Android:**
- Проверьте SHA256 fingerprint в assetlinks.json
- Убедитесь, что `android:autoVerify="true"` установлен
- Проверьте логи: `adb logcat | grep -i "deep"`

**Ссылка работает, но показывает ошибку:**
- Проверьте, что BACKEND_URL правильно настроен
- Убедитесь, что токен валиден (не истек, не использован)
- Проверьте логи backend сервиса

## API Endpoints

Для справки, доступные эндпоинты:

### Password Reset
```
POST   /api/v1/password-resets/request         - Запросить сброс пароля (public)
GET    /api/v1/password-resets/validate/:token - Проверить токен (public)
POST   /api/v1/password-resets/reset/:token    - Сбросить пароль (public)
GET    /reset-password/:token                   - HTML страница редиректа (public)
POST   /api/v1/admin/password-resets/initiate  - Инициировать сброс (admin)
```

### Invitations
```
POST   /api/v1/admin/invitations               - Создать приглашение (super admin)
GET    /api/v1/invitations/validate/:token     - Проверить приглашение (public)
POST   /api/v1/invitations/accept/:token       - Принять приглашение (public)
GET    /invite/:token                           - HTML страница редиректа (public)
GET    /api/v1/admin/invitations               - Список приглашений (super admin)
POST   /api/v1/admin/invitations/:id/resend    - Переотправить приглашение (super admin)
```

---

Если возникнут вопросы, обращайтесь к коду или документации React Native для Deep Linking:
- https://reactnative.dev/docs/linking
- https://reactnavigation.org/docs/deep-linking/
