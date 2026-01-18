# План: Система графиков работы (Work Schedules)

## Архитектурное решение

**Расширить calendar service** (не создавать отдельный микросервис).

**Обоснование:**
- Тесная интеграция с календарём (записи графика = события в календаре)
- Избежание распределённых транзакций
- Общие зависимости (Event, уведомления, права)
- Проще деплоить и поддерживать

## Структура расширения

```
services/calendar/
├── models/
│   ├── event.go              # Добавить ScheduleEntryID
│   └── schedule.go           # НОВЫЙ: Schedule, ScheduleEntry, ScheduleTemplate
├── repository/
│   └── schedule_repository.go # НОВЫЙ
├── usecase/
│   └── schedule_usecase.go    # НОВЫЙ
├── handlers/
│   └── schedule_handlers.go   # НОВЫЙ
├── import/
│   ├── docx_parser.go         # НОВЫЙ: парсер Word
│   └── table_detector.go      # НОВЫЙ: определение формата таблицы
└── clients/
    └── file_client.go         # НОВЫЙ
```

---

## Модели данных

### Schedule (График)
```go
type Schedule struct {
    ID             uint
    Title          string              // "Рабочий график Январь 2024"
    Description    string
    Type           ScheduleType        // work, paid_services, on_duty, shift, custom
    Visibility     ScheduleVisibility  // creator_only, management, participants
    CreatedBy      uint
    StartDate      time.Time
    EndDate        time.Time
    IsForAllUsers  bool
    DepartmentID   *uint
    Color          string              // HEX цвет для событий
    IsActive       bool
    TemplateID     *uint               // Связь с шаблоном (если создан из шаблона)
    ImportedFrom   *string             // Путь к файлу импорта
}
```

### ScheduleEntry (Запись в графике)
```go
type ScheduleEntry struct {
    ID           uint
    ScheduleID   uint
    UserID       uint
    Date         time.Time           // Дата
    StartTime    time.Time           // Время начала
    EndTime      time.Time           // Время окончания
    Title        string              // Опционально
    Description  string
    Location     string
    EventID      *uint               // Связь с календарным событием (создаётся автоматически)
    CreatedBy    uint
}
```

### ScheduleTemplate (Шаблон графика)
```go
type ScheduleTemplate struct {
    ID             uint
    Title          string              // "Стандартная рабочая неделя"
    Description    string
    Type           ScheduleType
    CreatedBy      uint
    DepartmentID   *uint
    Color          string
    IsActive       bool
}

type ScheduleTemplateEntry struct {
    ID           uint
    TemplateID   uint
    UserID       *uint               // nil = применить ко всем назначенным
    DayOfWeek    int                 // 0-6 (воскресенье-суббота)
    StartTime    string              // "09:00"
    EndTime      string              // "18:00"
    Title        string
    Location     string
}
```

### ScheduleAssignment (Назначение на график)
```go
type ScheduleAssignment struct {
    ID         uint
    ScheduleID uint
    UserID     uint
    AssignedBy uint
    AssignedAt time.Time
}
```

### Изменение Event
```go
// Добавить в существующую модель Event:
ScheduleEntryID *uint  // Связь с записью графика
```

---

## Система прав доступа

| Роль | Создание | Просмотр всех | Свои записи | Редактирование | Удаление |
|------|----------|---------------|-------------|----------------|----------|
| super_admin | + | + | + | + | + |
| admin | + | + | + | + | + |
| department_head | + свой отдел | + свой отдел | + | + свой отдел | + свой отдел |
| employee | - | - | + только свои | - | - |

**Ключевая логика:**
- Руководители видят полный график
- Обычные пользователи НЕ видят график напрямую
- Но если пользователь есть в графике → он видит своё событие в календаре

---

## API Endpoints

### Графики
```
POST   /api/v1/schedules                     # Создать график
GET    /api/v1/schedules                     # Список графиков (фильтр по правам)
GET    /api/v1/schedules/:id                 # Получить график
PUT    /api/v1/schedules/:id                 # Обновить график
DELETE /api/v1/schedules/:id                 # Удалить график
```

### Записи
```
POST   /api/v1/schedules/:id/entries         # Добавить записи (batch)
GET    /api/v1/schedules/:id/entries         # Получить записи
PUT    /api/v1/schedules/:id/entries/:eid    # Обновить запись
DELETE /api/v1/schedules/:id/entries/:eid    # Удалить запись
GET    /api/v1/schedules/my-entries          # Мои записи (для employee)
```

### Шаблоны
```
POST   /api/v1/schedule-templates            # Создать шаблон
GET    /api/v1/schedule-templates            # Список шаблонов
GET    /api/v1/schedule-templates/:id        # Получить шаблон
PUT    /api/v1/schedule-templates/:id        # Обновить шаблон
DELETE /api/v1/schedule-templates/:id        # Удалить шаблон
POST   /api/v1/schedule-templates/:id/apply  # Применить шаблон на период
```

### Импорт
```
POST   /api/v1/schedules/:id/import/preview  # Предпросмотр импорта (multipart)
POST   /api/v1/schedules/:id/import          # Выполнить импорт
```

---

## Импорт из Word

### Поддерживаемые форматы таблиц

**Формат 1: Стандартная таблица**
| ФИО | Дата | Начало | Конец | Примечание |
|-----|------|--------|-------|------------|
| Иванов И.И. | 15.01.2024 | 09:00 | 18:00 | Офис |

**Формат 2: Календарная сетка**
| Сотрудник | Пн 15.01 | Вт 16.01 | Ср 17.01 | ... |
|-----------|----------|----------|----------|-----|
| Иванов И.И. | 09:00-18:00 | 09:00-18:00 | - | ... |

### Алгоритм парсинга
1. Открыть .docx через `unidoc/unioffice`
2. Найти все таблицы в документе
3. Автоопределение формата по заголовкам
4. Парсинг строк с валидацией
5. Поиск пользователей по ФИО (fuzzy matching)
6. Возврат результата с ошибками

### Идентификация пользователей
- Поиск по ФИО с нечётким сравнением
- При множественных совпадениях → показать варианты
- Нераспознанные → вернуть ошибку с именем

---

## Интеграция с календарём

### При создании записи в графике
1. Создать ScheduleEntry
2. Автоматически создать Event:
   - Type: `schedule`
   - IsPrivate: `true` (видно только участнику)
   - Color: из настроек графика
   - Участник: только этот пользователь
3. Связать Event.ScheduleEntryID = entry.ID

### При обновлении записи
- Синхронно обновить связанный Event

### При удалении записи
- Удалить связанный Event

---

## Файлы для изменения/создания

### Новые файлы
- `services/calendar/models/schedule.go` — модели данных
- `services/calendar/repository/schedule_repository.go` — репозиторий
- `services/calendar/usecase/schedule_usecase.go` — бизнес-логика
- `services/calendar/handlers/schedule_handlers.go` — HTTP handlers
- `services/calendar/import/docx_parser.go` — парсер Word
- `services/calendar/import/table_detector.go` — определение формата
- `services/calendar/clients/file_client.go` — клиент file-service

### Изменяемые файлы
- `services/calendar/models/event.go` — добавить ScheduleEntryID, EventTypeSchedule
- `services/calendar/main.go` — добавить routes, миграции, DI

---

## Этапы реализации

### Этап 1: Модели и миграции
- Создать models/schedule.go
- Расширить models/event.go
- Добавить AutoMigrate в main.go

### Этап 2: Repository
- Создать schedule_repository.go с CRUD

### Этап 3: Usecase + интеграция с календарём
- Создать schedule_usecase.go
- Реализовать создание Event при создании ScheduleEntry
- Система прав доступа

### Этап 4: Handlers и API
- Создать schedule_handlers.go
- Добавить routes в main.go

### Этап 5: Шаблоны
- Модели ScheduleTemplate, ScheduleTemplateEntry
- CRUD для шаблонов
- Применение шаблона на период

### Этап 6: Импорт из Word
- Добавить зависимость unidoc/unioffice
- Создать docx_parser.go, table_detector.go
- Endpoints для preview и import
- Fuzzy matching по ФИО

---

## Верификация

### Тестирование
1. Создать график → проверить что записи появляются в календаре пользователей
2. Проверить что employee видит только свои события, не весь график
3. Проверить что department_head видит графики своего отдела
4. Импорт Word файла → проверить корректность парсинга
5. Применить шаблон → проверить генерацию записей

### Команды
```bash
# Запустить calendar service
cd services/calendar && go run main.go

# Проверить миграции
# Таблицы: schedules, schedule_entries, schedule_assignments, schedule_templates, schedule_template_entries

# Тестовые запросы
curl -X POST /api/v1/schedules -d '{"title":"Рабочий график","type":"work",...}'
curl -X GET /api/v1/schedules/my-entries
```
