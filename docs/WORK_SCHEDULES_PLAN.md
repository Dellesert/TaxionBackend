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
type ScheduleType string

const (
    ScheduleTypeWork          ScheduleType = "work"            // Рабочий график
    ScheduleTypePaidServices  ScheduleType = "paid_services"  // Платные услуги
    ScheduleTypeOnDuty        ScheduleType = "on_duty"        // Дежурства
    ScheduleTypeShift         ScheduleType = "shift"          // Сменный график
    ScheduleTypeCustom        ScheduleType = "custom"         // Кастомный
)

type ScheduleVisibility string

const (
    VisibilityCreatorOnly   ScheduleVisibility = "creator_only"   // Только создатель
    VisibilityManagement    ScheduleVisibility = "management"     // Руководство
    VisibilityParticipants  ScheduleVisibility = "participants"   // Участники
)

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

    // Настройки времени смен (по умолчанию)
    MorningStart   string              // "10:00" - начало утренней смены
    MorningEnd     string              // "14:00" - конец утренней смены
    EveningStart   string              // "14:00" - начало вечерней смены
    EveningEnd     string              // "18:00" - конец вечерней смены
}
```

### ScheduleEntry (Запись в графике)
```go
type ShiftType string

const (
    ShiftMorning    ShiftType = "morning"      // Утро (У)
    ShiftEvening    ShiftType = "evening"      // Вечер (В)
    ShiftFullDay    ShiftType = "full_day"     // Весь день (У/В)
    ShiftCustom     ShiftType = "custom"       // Кастомное время
)

type ScheduleEntry struct {
    ID           uint
    ScheduleID   uint
    UserID       uint
    Date         time.Time           // Дата
    ShiftType    ShiftType           // Тип смены: morning, evening, full_day, custom
    StartTime    time.Time           // Время начала (если custom)
    EndTime      time.Time           // Время окончания (если custom)
    Title        string              // Опционально
    Description  string
    Location     string
    EventID      *uint               // Связь с календарным событием (создаётся автоматически)
    CreatedBy    uint
}
```

**Логика времени:**
- Если `ShiftType = "morning"` → StartTime = 10:00, EndTime = 14:00 (или настраиваемое по графику)
- Если `ShiftType = "evening"` → StartTime = 14:00, EndTime = 18:00
- Если `ShiftType = "full_day"` → StartTime = 10:00, EndTime = 18:00
- Если `ShiftType = "custom"` → используются StartTime и EndTime из записи

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

**Формат 1: Таблица с временными слотами (декабрь 2025)**
```
График дежурных врачей СПБ ГБУЗ «ПНД № 5»
на декабрь 2025 года

| Дата      | С 9 до 14 часов | С 14 до 19 часов |
|-----------|-----------------|------------------|
| 01.12.25  | Клименченко     | Брежев           |
| 02.12.25  | Гермоненко      | Плотникова       |
```
- Две колонки времени: утренняя смена (9-14) и вечерняя смена (14-19)
- Каждая строка = одна дата
- В ячейках указаны фамилии врачей

**Формат 2: Таблица с обозначениями У/В (январь 2026)**
```
График дежурных врачей по ВК СПБ ГБУЗ «ПНД № 5»
на январь 2026 года

| Дата   | ФИО врача              | 10-14 | 14-18 |
|--------|------------------------|-------|-------|
| 12.01. | Карпунец В.В.          | У     | В     |
| 13.01. | Савельева О.В.         |       |       |
|        | Орлова Д.В             | У/В   |       |
| 14.01. | Ясюкевич Н.Ю.          | У     | В     |
|        | Плотникова И.А.        |       |       |
|        | Орлова Д.В             | У     |       |
```
- **У** = утренняя смена (10-14)
- **В** = вечерняя смена (14-18)
- **У/В** = обе смены (10-14 и 14-18)
- Несколько врачей могут быть назначены на одну дату

**Формат 3: Календарная сетка (январь 2026)**
```
ГРАФИК
учета рабочего времени по оказанию платных медицинских услуг за январь 2026 г.

| № п/п | Ф.И.О.      | 12 пн | 13 вт | 14 ср | 15 чт | 16 пт | 19 пн | 20 вт | 21 ср | ... |
|-------|-------------|-------|-------|-------|-------|-------|-------|-------|-------|-----|
| 1.    | Брежев      | У     |       |       |       |       | У     |       |       |     |
| 2.    | Быков       |       | У     |       |       |       |       |       | У     |     |
| 3.    | Гермоненко  |       |       |       | У     |       |       | У     |       |     |
| 7.    | Клименченко |       |       | В     |       |       |       |       | У     |     |
```
- Заголовки: дата + день недели
- **У** = утренняя смена
- **В** = вечерняя смена
- Пустая ячейка = выходной

### Алгоритм парсинга
1. Открыть .docx через `unidoc/unioffice`
2. Найти все таблицы в документе
3. Автоопределение формата по заголовкам:
   - Формат 1: поиск колонок с временем ("С 9 до 14", "С 14 до 19")
   - Формат 2: поиск колонок с временем ("10-14", "14-18") + колонка "ФИО"
   - Формат 3: поиск заголовков с датами и днями недели
4. Парсинг строк с валидацией:
   - Распознавание дат (DD.MM.YY, DD.MM.YYYY)
   - Парсинг обозначений: **У** → утро, **В** → вечер, **У/В** → обе смены
   - Поддержка пустых значений (выходной)
5. Поиск пользователей по ФИО (fuzzy matching)
6. Генерация ScheduleEntry для каждой смены
7. Возврат результата с ошибками и предупреждениями

### Идентификация пользователей
- Поиск по ФИО с нечётким сравнением
- При множественных совпадениях → показать варианты
- Нераспознанные → вернуть ошибку с именем

### Примеры парсинга

**Формат 1 → ScheduleEntry:**
```
Дата: 01.12.25, Колонка "С 9 до 14 часов": "Клименченко"
→ ScheduleEntry{Date: 2025-12-01, UserID: 123, ShiftType: "custom", StartTime: 09:00, EndTime: 14:00}

Дата: 01.12.25, Колонка "С 14 до 19 часов": "Брежев"
→ ScheduleEntry{Date: 2025-12-01, UserID: 456, ShiftType: "custom", StartTime: 14:00, EndTime: 19:00}
```

**Формат 2 → ScheduleEntry:**
```
Дата: 12.01, ФИО: "Карпунец В.В.", Колонка "10-14": "У", Колонка "14-18": "В"
→ ScheduleEntry{Date: 2026-01-12, UserID: 123, ShiftType: "morning", StartTime: 10:00, EndTime: 14:00}
→ ScheduleEntry{Date: 2026-01-12, UserID: 123, ShiftType: "evening", StartTime: 14:00, EndTime: 18:00}

Дата: 13.01, ФИО: "Орлова Д.В", Колонка "10-14": "У/В", Колонка "14-18": ""
→ ScheduleEntry{Date: 2026-01-13, UserID: 789, ShiftType: "full_day", StartTime: 10:00, EndTime: 18:00}
```

**Формат 3 → ScheduleEntry:**
```
ФИО: "Брежев", Колонка "12 пн": "У"
→ ScheduleEntry{Date: 2026-01-12, UserID: 123, ShiftType: "morning", StartTime: 10:00, EndTime: 14:00}

ФИО: "Клименченко", Колонка "14 ср": "В"
→ ScheduleEntry{Date: 2026-01-14, UserID: 456, ShiftType: "evening", StartTime: 14:00, EndTime: 18:00}
```

### Валидация и обработка ошибок

**Результат импорта (preview):**
```go
type ImportPreview struct {
    TotalRows       int
    SuccessCount    int
    ErrorCount      int
    WarningCount    int
    Entries         []ImportedEntry
    Errors          []ImportError
    Warnings        []ImportWarning
}

type ImportedEntry struct {
    RowNumber    int
    UserName     string
    UserID       *uint              // nil если пользователь не найден
    Date         time.Time
    ShiftType    ShiftType
    StartTime    time.Time
    EndTime      time.Time
    Status       string             // "success", "error", "warning"
}

type ImportError struct {
    RowNumber    int
    Field        string             // "user", "date", "time"
    Message      string
    Value        string             // Проблемное значение
}

type ImportWarning struct {
    RowNumber    int
    Message      string
}
```

**Примеры ошибок:**
- `{RowNumber: 5, Field: "user", Message: "Пользователь не найден", Value: "Иванов И."}`
- `{RowNumber: 12, Field: "date", Message: "Некорректный формат даты", Value: "32.13.2025"}`
- `{RowNumber: 8, Field: "time", Message: "Некорректное время", Value: "25:00"}`

**Примеры предупреждений:**
- `{RowNumber: 3, Message: "Найдено несколько пользователей: Иванов И.И. (ID: 123), Иванов И.П. (ID: 456)"}`
- `{RowNumber: 7, Message: "Пересечение смен: пользователь уже назначен на это время"}`

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
- Добавить зависимости:
  - `github.com/unidoc/unioffice` — парсинг .docx
  - `github.com/agnivade/levenshtein` — fuzzy matching ФИО
- Создать docx_parser.go, table_detector.go
- Endpoints для preview и import
- Fuzzy matching по ФИО

---

## Технологии и библиотеки

### Для импорта Word
```go
import (
    "github.com/unidoc/unioffice/document"
    "github.com/agnivade/levenshtein"
)
```

**Установка:**
```bash
go get github.com/unidoc/unioffice
go get github.com/agnivade/levenshtein
```

### Пример парсинга .docx
```go
doc, err := document.Open("schedule.docx")
if err != nil {
    return err
}
defer doc.Close()

for _, table := range doc.Tables() {
    for _, row := range table.Rows() {
        for _, cell := range row.Cells() {
            text := cell.Paragraphs()[0].Text()
            // Обработка ячейки
        }
    }
}
```

### Fuzzy matching ФИО
```go
func findUserByName(name string, users []User) (*User, error) {
    minDistance := math.MaxInt32
    var bestMatch *User

    for _, user := range users {
        distance := levenshtein.ComputeDistance(
            strings.ToLower(name),
            strings.ToLower(user.FullName),
        )

        if distance < minDistance {
            minDistance = distance
            bestMatch = &user
        }
    }

    // Порог схожести (30% от длины)
    threshold := len(name) / 3
    if minDistance <= threshold {
        return bestMatch, nil
    }

    return nil, fmt.Errorf("user not found: %s", name)
}
```

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
