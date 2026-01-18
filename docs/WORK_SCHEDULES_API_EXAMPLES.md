# Work Schedules API Examples

## Authentication
All requests require JWT token in Authorization header:
```
Authorization: Bearer <your-jwt-token>
```

## 1. Schedules

### Create Schedule
```http
POST /api/v1/schedules
Content-Type: application/json

{
  "title": "График дежурств - Январь 2026",
  "description": "Месячный график работы отделения",
  "type": "on_duty",
  "visibility": "management",
  "start_date": "2026-01-01T00:00:00Z",
  "end_date": "2026-01-31T23:59:59Z",
  "morning_start": "10:00",
  "morning_end": "14:00",
  "evening_start": "14:00",
  "evening_end": "18:00",
  "color": "#4CAF50",
  "department_id": 1
}
```

**Response:**
```json
{
  "message": "Schedule created successfully",
  "schedule": {
    "id": 1,
    "title": "График дежурств - Январь 2026",
    "type": "on_duty",
    "visibility": "management",
    "created_by": 5,
    "start_date": "2026-01-01T00:00:00Z",
    "end_date": "2026-01-31T23:59:59Z",
    "is_active": true,
    "color": "#4CAF50",
    "created_at": "2026-01-18T10:00:00Z"
  },
  "request_id": "..."
}
```

### Get Schedules (with filters)
```http
GET /api/v1/schedules?type=on_duty&is_active=true&limit=20&offset=0
```

**Response:**
```json
{
  "schedules": [...],
  "total": 5,
  "limit": 20,
  "offset": 0,
  "request_id": "..."
}
```

### Get Schedule Details
```http
GET /api/v1/schedules/1
```

### Update Schedule
```http
PUT /api/v1/schedules/1
Content-Type: application/json

{
  "title": "График дежурств - Январь 2026 (обновлённый)",
  "is_active": false
}
```

### Delete Schedule
```http
DELETE /api/v1/schedules/1
```

## 2. Schedule Entries

### Create Single Entry
```http
POST /api/v1/schedules/1/entries
Content-Type: application/json

{
  "user_id": 10,
  "date": "2026-01-15",
  "shift_type": "morning",
  "title": "Дежурство в приёмной",
  "location": "Корпус А"
}
```

**Response:**
```json
{
  "message": "Schedule entry created successfully",
  "entry": {
    "id": 100,
    "schedule_id": 1,
    "user_id": 10,
    "date": "2026-01-15T00:00:00Z",
    "shift_type": "morning",
    "start_time": "2026-01-15T10:00:00Z",
    "end_time": "2026-01-15T14:00:00Z",
    "title": "Дежурство в приёмной",
    "event_id": 205,
    "created_at": "2026-01-18T10:05:00Z"
  },
  "request_id": "..."
}
```

### Create Batch Entries
```http
POST /api/v1/schedules/1/entries
Content-Type: application/json

{
  "entries": [
    {
      "user_id": 10,
      "date": "2026-01-15",
      "shift_type": "morning"
    },
    {
      "user_id": 11,
      "date": "2026-01-15",
      "shift_type": "evening"
    },
    {
      "user_id": 10,
      "date": "2026-01-16",
      "shift_type": "full_day"
    }
  ]
}
```

**Response:**
```json
{
  "message": "3 schedule entries created successfully",
  "entries": [...],
  "request_id": "..."
}
```

### Get Schedule Entries (with filters)
```http
GET /api/v1/schedules/1/entries?user_id=10&start_date=2026-01-01&end_date=2026-01-31&shift_type=morning
```

### Get My Schedule Entries
```http
GET /api/v1/schedules/my-entries?start_date=2026-01-01&end_date=2026-01-31
```

### Update Entry
```http
PUT /api/v1/schedules/1/entries/100
Content-Type: application/json

{
  "shift_type": "full_day",
  "title": "Дежурство весь день"
}
```

### Delete Entry
```http
DELETE /api/v1/schedules/1/entries/100
```

## 3. Templates

### Create Template
```http
POST /api/v1/schedule-templates
Content-Type: application/json

{
  "title": "Стандартный график 2/2",
  "description": "Два дня работы, два дня отдыха",
  "type": "on_duty",
  "color": "#2196F3",
  "department_id": 1
}
```

**Response:**
```json
{
  "message": "Template created successfully",
  "template": {
    "id": 1,
    "title": "Стандартный график 2/2",
    "type": "on_duty",
    "created_by": 5,
    "is_active": true,
    "color": "#2196F3",
    "created_at": "2026-01-18T10:10:00Z"
  },
  "request_id": "..."
}
```

### Add Entry to Template
```http
POST /api/v1/schedule-templates/1/entries
Content-Type: application/json

{
  "user_id": 10,
  "day_of_week": 1,
  "start_time": "10:00",
  "end_time": "18:00",
  "title": "Понедельник - полный день"
}
```

### Add General Entry (for all users)
```http
POST /api/v1/schedule-templates/1/entries
Content-Type: application/json

{
  "day_of_week": 3,
  "start_time": "10:00",
  "end_time": "14:00",
  "title": "Среда - утро"
}
```

**Note:** `user_id` is optional. If omitted, entry applies to all users.

### Get Template with Entries
```http
GET /api/v1/schedule-templates/1
```

**Response:**
```json
{
  "template": {
    "id": 1,
    "title": "Стандартный график 2/2",
    "entries": [
      {
        "id": 1,
        "template_id": 1,
        "user_id": 10,
        "day_of_week": 1,
        "start_time": "10:00",
        "end_time": "18:00"
      }
    ]
  },
  "request_id": "..."
}
```

### Apply Template to Schedule
```http
POST /api/v1/schedule-templates/1/apply?schedule_id=1
Content-Type: application/json

{
  "start_date": "2026-01-01T00:00:00Z",
  "end_date": "2026-01-31T23:59:59Z",
  "user_ids": [10, 11, 12]
}
```

**Response:**
```json
{
  "message": "Template applied successfully",
  "entries_created": 45,
  "request_id": "..."
}
```

**Note:** If `user_ids` is omitted, template applies to all assigned users.

### Get Templates (with filters)
```http
GET /api/v1/schedule-templates?type=on_duty&is_active=true
```

### Update Template
```http
PUT /api/v1/schedule-templates/1
Content-Type: application/json

{
  "title": "График 2/2 (обновлённый)",
  "is_active": false
}
```

### Delete Template Entry
```http
DELETE /api/v1/schedule-templates/1/entries/1
```

### Delete Template
```http
DELETE /api/v1/schedule-templates/1
```

## 4. Import from Word

### Get Supported Formats
```http
GET /api/v1/schedules/import/formats
```

**Response:**
```json
{
  "formats": [
    {
      "name": "Time Slots Format",
      "description": "Table with dates in first column and time slots as headers",
      "example": "декабрь 2025 format"
    },
    {
      "name": "У/В Designation Format",
      "description": "Table with names in first column and dates as headers",
      "example": "январь 2026 format"
    },
    {
      "name": "Calendar Grid Format",
      "description": "Calendar-style grid with dates 1-31 as columns",
      "example": "Standard calendar layout"
    }
  ],
  "file_types": [".docx", ".doc"],
  "request_id": "..."
}
```

### Preview Import
```http
POST /api/v1/schedules/import
Content-Type: application/json

{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "График дежурств - Декабрь 2025",
  "description": "Импортирован из Word",
  "type": "on_duty",
  "start_date": "2025-12-01T00:00:00Z",
  "end_date": "2025-12-31T23:59:59Z",
  "preview": true
}
```

**Response:**
```json
{
  "preview": {
    "schedule": {
      "title": "График дежурств - Декабрь 2025",
      "type": "on_duty"
    },
    "entries": [...],
    "entries_count": 62,
    "users": [
      {
        "name": "Иванов И.И.",
        "user_id": 10,
        "match_score": 0.95,
        "is_unmatched": false
      },
      {
        "name": "Петров П.П.",
        "user_id": null,
        "match_score": 0,
        "is_unmatched": true
      }
    ],
    "warnings": [
      "No match found for user: Петров П.П."
    ]
  },
  "request_id": "..."
}
```

### Actual Import
```http
POST /api/v1/schedules/import
Content-Type: application/json

{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "График дежурств - Декабрь 2025",
  "type": "on_duty",
  "start_date": "2025-12-01T00:00:00Z",
  "end_date": "2025-12-31T23:59:59Z",
  "preview": false
}
```

**Response:**
```json
{
  "message": "Schedule imported successfully",
  "result": {
    "schedule": {
      "id": 5,
      "title": "График дежурств - Декабрь 2025",
      "imported_from": "график_декабрь_2025.docx"
    },
    "entries_count": 58,
    "imported_from": "график_декабрь_2025.docx",
    "warnings": [
      "Skipping entry for unmatched user: Петров П.П."
    ]
  },
  "request_id": "..."
}
```

## Common Filter Parameters

### Schedules
- `type` - Schedule type (work, paid_services, on_duty, shift, custom)
- `is_active` - Boolean (true/false)
- `department_id` - Department ID
- `limit` - Page size (default: 20)
- `offset` - Page offset (default: 0)

### Schedule Entries
- `user_id` - Filter by user
- `start_date` - Filter by date range start
- `end_date` - Filter by date range end
- `shift_type` - Shift type (morning, evening, full_day, custom)
- `limit` - Page size
- `offset` - Page offset

### Templates
- `type` - Template type
- `is_active` - Boolean
- `department_id` - Department ID
- `limit` - Page size
- `offset` - Page offset

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid request body",
  "details": "start_date must be before end_date",
  "request_id": "..."
}
```

### 401 Unauthorized
```json
{
  "error": "Unauthorized",
  "request_id": "..."
}
```

### 404 Not Found
```json
{
  "error": "Failed to get schedule",
  "details": "record not found",
  "request_id": "..."
}
```

### 500 Internal Server Error
```json
{
  "error": "Failed to create schedule",
  "details": "database connection error",
  "request_id": "..."
}
```

## Notes

1. All dates should be in ISO 8601 format: `2026-01-15T10:00:00Z`
2. Time strings use 24-hour format: `"14:00"`
3. Day of week: 0 = Sunday, 1 = Monday, ..., 6 = Saturday
4. All responses include `request_id` for tracking
5. Shift times are automatically calculated based on `shift_type` if not provided
6. Creating a schedule entry automatically creates a linked calendar event
