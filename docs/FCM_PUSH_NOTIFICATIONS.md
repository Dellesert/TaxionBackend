# Firebase Cloud Messaging (FCM) Push Notifications

## Обзор

Реализована полноценная система push-уведомлений для React Native приложения с использованием Firebase Cloud Messaging (FCM).

## Что уже реализовано ✅

### 1. Модели данных

- **DeviceToken** ([models/device_token.go](../services/notification/models/device_token.go))
  - Хранение FCM токенов устройств
  - Поддержка iOS, Android, Web
  - Отслеживание активности и метаданных устройств

### 2. Push Provider

- **PushProvider интерфейс** ([push/provider.go](../services/notification/push/provider.go))
  - Абстракция для разных провайдеров
  - Поддержка batch отправки
  - Topics для групповых уведомлений

- **FCMProvider** ([push/fcm_provider.go](../services/notification/push/fcm_provider.go))
  - Полная реализация для FCM
  - iOS специфичные настройки (badge, sound, category, silent push)
  - Android специфичные настройки (channel, color, TTL)
  - Data payload для deep linking

### 3. Repository & Usecase

- **DeviceTokenRepository** ([repository/device_token_repository.go](../services/notification/repository/device_token_repository.go))
  - CRUD операции с токенами
  - Bulk queries для производительности
  - Cleanup методы для неактивных устройств

- **DeviceUsecase** ([usecase/device_usecase.go](../services/notification/usecase/device_usecase.go))
  - Бизнес-логика управления устройствами
  - Валидация токенов через FCM

- **NotificationUsecase** - Расширен для отправки push
  - `sendPushNotification()` - отправка push-уведомлений
  - Platform-specific конфигурации
  - Автоматическое определение action по типу

### 4. API Endpoints

- **DeviceHandler** ([handlers/device_handler.go](../services/notification/handlers/device_handler.go))
  - `POST /api/v1/devices` - Регистрация устройства
  - `GET /api/v1/devices` - Получить список устройств
  - `GET /api/v1/devices/:id` - Получить устройство
  - `PUT /api/v1/devices/:id` - Обновить устройство
  - `DELETE /api/v1/devices/:id` - Удалить устройство
  - `POST /api/v1/devices/:id/deactivate` - Деактивировать
  - `POST /api/v1/devices/validate` - Валидация токена
  - `GET /api/v1/devices/stats` - Статистика

## Типы Push-уведомлений

### Сообщения (Messages)
- Новое личное сообщение - `NotificationTypeMessage`
- Упоминание (@mention) - `NotificationTypeMention`
- Action: `open_chat`
- Channel: `messages` / `mentions`

### Задачи (Tasks)
- Новая задача - `NotificationTypeTask`
- Напоминание о дедлайне - `NotificationTypeReminder`
- Action: `open_task`
- Channel: `tasks` / `reminders`

### Календарь (Calendar)
- Напоминание о событии - `NotificationTypeCalendar`
- Action: `open_event`
- Channel: `calendar`

### Опросы (Polls)
- Новый опрос - `NotificationTypePoll`
- Action: `open_poll`
- Channel: `polls`

### Системные (System)
- Объявления - `NotificationTypeAnnounce`
- Системные уведомления - `NotificationTypeSystem`
- Channels: `announcements` / `system`

## Настройка Firebase

### 1. Создание проекта Firebase

1. Перейдите на [Firebase Console](https://console.firebase.google.com/)
2. Создайте новый проект или выберите существующий
3. Добавьте приложения (iOS и/или Android)

### 2. Получение Service Account Key

1. В Firebase Console откройте **Project Settings**
2. Перейдите на вкладку **Service Accounts**
3. Нажмите **Generate New Private Key**
4. Сохраните JSON файл в безопасное место

### 3. Настройка Backend

#### a. Разместите credentials file

```bash
# Создайте директорию для credentials в корне проекта
mkdir -p credentials

# Скопируйте файл service account
cp /path/to/downloaded/firebase-adminsdk-xxxxx.json credentials/firebase-credentials.json

# Установите права доступа
chmod 600 credentials/firebase-credentials.json
```

#### b. Обновите .env файл

```env
# FCM Push Notifications
FCM_ENABLED=true
FCM_CREDENTIALS_FILE=/app/credentials/firebase-credentials.json
FCM_PROJECT_ID=your-actual-project-id
```

#### c. Обновите docker-compose.yml

Раскомментируйте строку в `docker-compose.yml` для монтирования credentials:

```yaml
notification-service:
  volumes:
    - ./logs:/app/logs
    # Раскомментируйте следующую строку:
    - ./credentials/firebase-credentials.json:/app/credentials/firebase-credentials.json:ro
```

### 4. Обновите main.go

В `services/notification/main.go` добавьте инициализацию FCM:

```go
import (
	"tachyon-messenger/services/notification/push"
	"tachyon-messenger/services/notification/repository"
)

// После инициализации базы данных добавьте:

// Добавьте миграцию для device tokens
if err := db.Migrate(
	&models.Notification{},
	&models.NotificationDelivery{},
	&models.EmailTemplate{},
	&models.UserNotificationPreference{},
	&models.NotificationTemplate{},
	&models.DeviceToken{}, // <-- ДОБАВИТЬ ЭТО
); err != nil {
	log.Fatalf("Failed to run GORM migrations: %v", err)
}

// Инициализация Push Provider (FCM)
var pushProvider push.PushProvider
if os.Getenv("FCM_ENABLED") == "true" {
	fcmConfig := &push.PushConfig{
		CredentialsFile: os.Getenv("FCM_CREDENTIALS_FILE"),
		ProjectID:       os.Getenv("FCM_PROJECT_ID"),
		Provider:        "fcm",
	}

	var err error
	pushProvider, err = push.NewFCMProvider(fcmConfig)
	if err != nil {
		log.Warnf("Failed to initialize FCM provider: %v", err)
		log.Info("Push notifications will be disabled")
	} else {
		log.Info("FCM push provider initialized successfully")
	}
} else {
	log.Info("FCM push notifications disabled by configuration")
}

// Инициализация repositories
notificationRepo := repository.NewNotificationRepository(db)
deviceRepo := repository.NewDeviceTokenRepository(db) // <-- ДОБАВИТЬ ЭТО

// Инициализация usecases
notificationUC := usecase.NewNotificationUsecase(
	notificationRepo,
	deviceRepo,      // <-- ДОБАВИТЬ ЭТО
	emailSender,
	pushProvider,    // <-- ДОБАВИТЬ ЭТО
)
deviceUC := usecase.NewDeviceUsecase(deviceRepo, pushProvider) // <-- ДОБАВИТЬ ЭТО

// Инициализация handlers
notificationHandler := handlers.NewNotificationHandler(notificationUC)
deviceHandler := handlers.NewDeviceHandler(deviceUC) // <-- ДОБАВИТЬ ЭТО
```

### 5. Добавьте routes в setupRoutes

```go
// В функции setupRoutes добавьте:

// Device management endpoints
devices := v1.Group("/devices")
devices.Use(middleware.AuthMiddleware())
{
	devices.POST("", deviceHandler.RegisterDevice)              // POST /api/v1/devices
	devices.GET("", deviceHandler.GetUserDevices)               // GET /api/v1/devices
	devices.GET("/stats", deviceHandler.GetDeviceStats)         // GET /api/v1/devices/stats
	devices.POST("/validate", deviceHandler.ValidateToken)      // POST /api/v1/devices/validate
	devices.GET("/:id", deviceHandler.GetDevice)                // GET /api/v1/devices/:id
	devices.PUT("/:id", deviceHandler.UpdateDevice)             // PUT /api/v1/devices/:id
	devices.DELETE("/:id", deviceHandler.DeleteDevice)          // DELETE /api/v1/devices/:id
	devices.POST("/:id/deactivate", deviceHandler.DeactivateDevice) // POST /api/v1/devices/:id/deactivate
}
```

## Интеграция с React Native

### 1. Установка зависимостей

```bash
npm install @react-native-firebase/app @react-native-firebase/messaging
# или
yarn add @react-native-firebase/app @react-native-firebase/messaging
```

### 2. Конфигурация Firebase в приложении

#### iOS (ios/Podfile)
```ruby
# Add to Podfile
pod 'Firebase/Messaging'
```

Разместите `GoogleService-Info.plist` в `ios/YourApp/`

#### Android (android/build.gradle)
```gradle
buildscript {
  dependencies {
    classpath 'com.google.gms:google-services:4.3.15'
  }
}
```

Разместите `google-services.json` в `android/app/`

### 3. Регистрация устройства

```typescript
import messaging from '@react-native-firebase/messaging';
import axios from 'axios';

// Запросить разрешение на уведомления
async function requestUserPermission() {
  const authStatus = await messaging().requestPermission();
  const enabled =
    authStatus === messaging.AuthorizationStatus.AUTHORIZED ||
    authStatus === messaging.AuthorizationStatus.PROVISIONAL;

  if (enabled) {
    console.log('Authorization status:', authStatus);
    return true;
  }
  return false;
}

// Получить FCM токен и зарегистрировать устройство
async function registerDevice() {
  const hasPermission = await requestUserPermission();

  if (!hasPermission) {
    console.log('Push notification permission denied');
    return;
  }

  // Получить FCM токен
  const fcmToken = await messaging().getToken();
  console.log('FCM Token:', fcmToken);

  // Получить информацию об устройстве
  const deviceInfo = {
    token: fcmToken,
    platform: Platform.OS, // 'ios' или 'android'
    device_id: DeviceInfo.getUniqueId(),
    device_name: await DeviceInfo.getDeviceName(),
    app_version: DeviceInfo.getVersion(),
    os_version: DeviceInfo.getSystemVersion(),
  };

  // Отправить на backend
  try {
    const response = await axios.post(
      'https://your-api.com/api/v1/devices',
      deviceInfo,
      {
        headers: {
          Authorization: `Bearer ${yourAuthToken}`,
        },
      }
    );
    console.log('Device registered:', response.data);
  } catch (error) {
    console.error('Failed to register device:', error);
  }
}
```

### 4. Обработка уведомлений

```typescript
import messaging from '@react-native-firebase/messaging';
import { useNavigation } from '@react-navigation/native';

// Обработка foreground уведомлений
useEffect(() => {
  const unsubscribe = messaging().onMessage(async remoteMessage => {
    console.log('Foreground notification:', remoteMessage);

    // Показать локальное уведомление
    // или обновить UI
  });

  return unsubscribe;
}, []);

// Обработка background/quit уведомлений
useEffect(() => {
  messaging().onNotificationOpenedApp(remoteMessage => {
    console.log('Notification opened app from background:', remoteMessage);
    handleNotificationNavigation(remoteMessage);
  });

  messaging()
    .getInitialNotification()
    .then(remoteMessage => {
      if (remoteMessage) {
        console.log('Notification opened app from quit state:', remoteMessage);
        handleNotificationNavigation(remoteMessage);
      }
    });
}, []);

// Навигация по deep link из уведомления
function handleNotificationNavigation(remoteMessage) {
  const { data } = remoteMessage;

  switch (data.action) {
    case 'open_chat':
      navigation.navigate('Chat', { chatId: data.chat_id });
      break;
    case 'open_task':
      navigation.navigate('Task', { taskId: data.task_id });
      break;
    case 'open_event':
      navigation.navigate('Event', { eventId: data.event_id });
      break;
    case 'open_poll':
      navigation.navigate('Poll', { pollId: data.poll_id });
      break;
    default:
      navigation.navigate('Home');
  }
}
```

### 5. Android Notification Channels

Создайте каналы уведомлений для Android (в `MainActivity.java` или при запуске приложения):

```typescript
import notifee, { AndroidImportance } from '@notifee/react-native';

async function createNotificationChannels() {
  // Messages channel
  await notifee.createChannel({
    id: 'messages',
    name: 'Messages',
    importance: AndroidImportance.HIGH,
    sound: 'default',
  });

  // Tasks channel
  await notifee.createChannel({
    id: 'tasks',
    name: 'Tasks',
    importance: AndroidImportance.DEFAULT,
  });

  // Calendar channel
  await notifee.createChannel({
    id: 'calendar',
    name: 'Calendar Events',
    importance: AndroidImportance.HIGH,
  });

  // ... остальные каналы
}
```

## Data Payload Structure

Каждое push-уведомление содержит data payload для навигации:

```json
{
  "notification": {
    "title": "Новое сообщение",
    "body": "Привет! Как дела?"
  },
  "data": {
    "type": "message",
    "chat_id": "123",
    "message_id": "456",
    "action": "open_chat",
    "action_url": "tachyon://chat/123"
  }
}
```

## Приоритеты уведомлений

| Priority | iOS | Android | Использование |
|----------|-----|---------|---------------|
| **critical** | Critical Alert | HIGH | Срочные события, просроченные задачи |
| **high** | Active | HIGH | Новые сообщения, упоминания |
| **medium** | Active | DEFAULT | Обычные уведомления |
| **low** | Passive | LOW | Системные уведомления |

## Настройки пользователя

Пользователи могут управлять уведомлениями через `UserNotificationPreference`:

```json
{
  "notification_type": "message",
  "in_app_enabled": true,
  "email_enabled": true,
  "push_enabled": true,
  "sms_enabled": false,
  "min_priority": "low",
  "quiet_hours_start": 22,
  "quiet_hours_end": 8,
  "weekend_enabled": true
}
```

## Тестирование

### 1. Тестирование через API

```bash
# Зарегистрировать тестовое устройство
curl -X POST https://your-api.com/api/v1/devices \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "token": "YOUR_FCM_TOKEN",
    "platform": "android",
    "device_name": "Test Device"
  }'

# Отправить тестовое уведомление
curl -X POST https://your-api.com/api/v1/admin/notifications/send \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "type": "message",
    "title": "Test Notification",
    "message": "This is a test",
    "priority": "high",
    "channels": ["push", "in_app"]
  }'
```

### 2. Проверка статистики

```bash
curl -X GET https://your-api.com/api/v1/devices/stats \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Troubleshooting

### Push не приходят

1. **Проверьте FCM credentials**
   ```bash
   # Убедитесь, что файл credentials доступен
   cat $FCM_CREDENTIALS_FILE
   ```

2. **Проверьте логи**
   ```bash
   docker-compose logs -f notification-service | grep -i "fcm\|push"
   ```

3. **Проверьте устройства пользователя**
   ```bash
   curl -X GET https://your-api.com/api/v1/devices \
     -H "Authorization: Bearer YOUR_TOKEN"
   ```

4. **Проверьте настройки пользователя**
   ```bash
   curl -X GET https://your-api.com/api/v1/notifications/preferences \
     -H "Authorization: Bearer YOUR_TOKEN"
   ```

### Токен становится невалидным

FCM автоматически обновляет токены. Обработайте это в React Native:

```typescript
useEffect(() => {
  const unsubscribe = messaging().onTokenRefresh(async (newToken) => {
    console.log('FCM Token refreshed:', newToken);
    // Обновите токен на backend
    await updateDeviceToken(newToken);
  });

  return unsubscribe;
}, []);
```

## Производительность

- **Batch отправка**: До 500 устройств за раз
- **Timeout**: 30 секунд на batch
- **Retry**: Автоматический retry через worker
- **Rate limiting**: Настраивается через user preferences

## Безопасность

- ✅ Токены хранятся с unique index
- ✅ Валидация токенов через FCM dry-run
- ✅ Автоматическая деактивация старых токенов
- ✅ Проверка ownership устройств
- ✅ HTTPS для всех API endpoints

## Дополнительные ресурсы

- [Firebase Cloud Messaging Documentation](https://firebase.google.com/docs/cloud-messaging)
- [React Native Firebase](https://rnfirebase.io/)
- [FCM HTTP v1 API](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages)
