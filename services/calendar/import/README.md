# Schedule Import from Word Documents

This package provides functionality to import work schedules from Word (.docx, .doc) documents into the system.

## Supported Formats

### Format 1: Time Slots Format
Table with dates in the first column and time slots as headers.

**Example:**
```
Date       | 10:00-14:00 | 14:00-18:00
-----------|-------------|-------------
1 декабря  | Иванов И.И. | Петров П.П.
2 декабря  | Сидоров С.С.| Иванов И.И.
```

### Format 2: У/В Designation Format
Table with names in the first column and dates as headers. Cells contain:
- `У` - morning shift (утро)
- `В` - evening shift (вечер)

**Example:**
```
ФИО         | 1  | 2  | 3  | 4
------------|----|----|----|----|
Иванов И.И. | У  | В  | УВ | -
Петров П.П. | В  | У  | -  | УВ
```

### Format 3: Calendar Grid Format
Calendar-style grid with dates 1-31 as columns and names as rows.

## Features

- **Auto-detection** of table format
- **Fuzzy matching** of user names using Levenshtein distance
- **Preview mode** to see import results before creating
- **Batch import** for efficient data insertion
- **Warnings** for unmatched users and invalid data
- **Support** for multiple table formats

## API Endpoints

### Import Schedule
```http
POST /api/v1/schedules/import
Content-Type: application/json

{
  "file_id": "uuid-from-file-service",
  "title": "График дежурств - Декабрь 2025",
  "description": "Месячный график",
  "type": "on_duty",
  "start_date": "2025-12-01T00:00:00Z",
  "end_date": "2025-12-31T23:59:59Z",
  "preview": false
}
```

### Preview Import
Same as above, but set `preview: true` to get a preview without creating the schedule.

### Get Supported Formats
```http
GET /api/v1/schedules/import/formats
```

## Implementation Details

### Components

1. **table_detector.go** - Detects table format and extracts month/year
2. **docx_parser.go** - Parses document and creates schedule entries
3. **schedule_import_usecase.go** - Business logic for import
4. **schedule_import_handlers.go** - HTTP handlers for import endpoints
5. **file_client.go** - Integration with file-service

### User Matching

The system uses fuzzy string matching (Levenshtein distance) to match names from the document to actual users in the system:

- Minimum match score: 60%
- Tries multiple name formats:
  - "FirstName LastName"
  - "LastName FirstName"
  - "LastName" only

Unmatched users are:
- Included in warnings
- Skipped during actual import

### Default Shift Times

- Morning shift (У): 10:00-14:00
- Evening shift (В): 14:00-18:00
- Full day: 10:00-18:00

These can be customized when creating the schedule.

## Usage Example

1. Upload Word document to file-service
2. Get file_id from upload response
3. Call preview endpoint to verify parsing
4. Review matched users and warnings
5. Call import endpoint to create schedule

## Dependencies

- `github.com/unidoc/unioffice` - Word document parsing
- `github.com/agnivade/levenshtein` - Fuzzy string matching
