package importschedule

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TableFormat represents the detected table format type
type TableFormat int

const (
	// FormatTimeSlots - Format 1: Time slots with names (декабрь 2025)
	FormatTimeSlots TableFormat = iota
	// FormatDesignation - Format 2: У/В designation (январь 2026)
	FormatDesignation
	// FormatCalendarGrid - Format 3: Calendar grid (unknown format)
	FormatCalendarGrid
	// FormatUnknown - Unknown format
	FormatUnknown
)

// TableDetector detects table format in Word documents
type TableDetector struct {
	monthPattern *regexp.Regexp
	timePattern  *regexp.Regexp
}

// NewTableDetector creates a new table detector
func NewTableDetector() *TableDetector {
	return &TableDetector{
		monthPattern: regexp.MustCompile(`(?i)(январь|февраль|март|апрель|май|июнь|июль|август|сентябрь|октябрь|ноябрь|декабрь)\s*(\d{4})`),
		// Matches both "10:00-14:00" and "С 9 до 14 часов"
		timePattern: regexp.MustCompile(`(\d{1,2}[:\.]\d{2}\s*[-–—]\s*\d{1,2}[:\.]\d{2})|([сСcC]\s*\d{1,2}\s*до\s*\d{1,2})`),
	}
}

// DetectFormat detects the table format in a Word document
func (d *TableDetector) DetectFormat(doc *DocxDocument) (TableFormat, error) {
	if len(doc.Tables) == 0 {
		return FormatUnknown, fmt.Errorf("no tables found in document (found %d paragraphs)", len(doc.Paragraphs))
	}

	// Try each table until we find one that matches a format
	for tableIdx, table := range doc.Tables {
		if len(table.Rows) < 2 {
			continue
		}

		// Get all text from first few rows for analysis
		var allRowsText strings.Builder
		for i := 0; i < min(5, len(table.Rows)); i++ {
			allRowsText.WriteString(d.extractRowText(table.Rows[i]))
			allRowsText.WriteString(" ")
		}
		tableText := allRowsText.String()

		// Check for Format 1: Time slots format (look for time patterns like "10:00-14:00")
		if d.hasTimeSlots(tableText) {
			// Verify it's really a time slots table by checking header
			if tableIdx == 0 || d.hasTimeSlots(d.extractRowText(table.Rows[0])) {
				return FormatTimeSlots, nil
			}
		}

		// Check for Format 2: У/В designation (check multiple rows)
		designationCount := 0
		for i := 1; i < min(10, len(table.Rows)); i++ {
			rowText := d.extractRowText(table.Rows[i])
			if d.hasDesignationInRow(rowText) {
				designationCount++
			}
		}
		if designationCount >= 2 {
			return FormatDesignation, nil
		}

		// Check for Format 3: Calendar grid (header with numbers 1-31)
		if d.hasCalendarGrid(table.Rows[0]) {
			return FormatCalendarGrid, nil
		}
	}

	// If no specific format detected, try to determine based on structure
	// If there are tables, assume the most common format (designation)
	if len(doc.Tables) > 0 && len(doc.Tables[0].Rows) >= 2 {
		// Default to designation format if we have a table with data
		return FormatDesignation, nil
	}

	return FormatUnknown, fmt.Errorf("unable to detect table format (found %d tables)", len(doc.Tables))
}

// hasTimeSlots checks if text has time slot patterns
func (d *TableDetector) hasTimeSlots(text string) bool {
	// Format 1 has columns like "10:00-14:00", "14:00-18:00"
	matches := d.timePattern.FindAllString(text, -1)
	return len(matches) >= 1
}

// hasDesignationInRow checks if a single row has У/В markers
func (d *TableDetector) hasDesignationInRow(rowText string) bool {
	normalized := strings.ToUpper(rowText)
	countU := strings.Count(normalized, "У")
	countV := strings.Count(normalized, "В")
	// Row should have at least a few markers to be considered designation format
	return (countU + countV) >= 2
}

// hasDesignation checks if row has У/В designation
func (d *TableDetector) hasDesignation(rowText string) bool {
	// Format 2 has cells with "У" or "В" or both
	normalized := strings.ToUpper(rowText)
	hasU := strings.Contains(normalized, "У")
	hasV := strings.Contains(normalized, "В")

	// Should have at least a few У or В markers
	countU := strings.Count(normalized, "У")
	countV := strings.Count(normalized, "В")

	return (hasU || hasV) && (countU+countV > 2)
}

// hasCalendarGrid checks if header is a calendar grid (1-31 columns)
func (d *TableDetector) hasCalendarGrid(headerRow DocxRow) bool {
	cells := headerRow.Cells

	// Calendar grid typically has many columns (dates)
	if len(cells) < 10 {
		return false
	}

	// Check if cells contain sequential numbers
	numberCount := 0
	for i := 0; i < min(15, len(cells)); i++ {
		cellText := strings.TrimSpace(cells[i].GetText())
		if matched, _ := regexp.MatchString(`^\d{1,2}$`, cellText); matched {
			numberCount++
		}
	}

	// At least 5 numbers found
	return numberCount >= 5
}

// extractRowText extracts all text from a row
func (d *TableDetector) extractRowText(row DocxRow) string {
	var text strings.Builder
	for _, cell := range row.Cells {
		text.WriteString(cell.GetText())
		text.WriteString(" ")
	}
	return text.String()
}

// extractCellText extracts text from a cell
func (d *TableDetector) extractCellText(cell DocxCell) string {
	return cell.GetText()
}

// ExtractMonthYear extracts month and year from document text
func (d *TableDetector) ExtractMonthYear(doc *DocxDocument) (time.Month, int, error) {
	// Search in paragraphs
	for _, para := range doc.Paragraphs {
		month, year, ok := d.parseMonthYear(para.Text)
		if ok {
			return month, year, nil
		}
	}

	// Search in tables
	for _, table := range doc.Tables {
		for _, row := range table.Rows {
			text := d.extractRowText(row)
			month, year, ok := d.parseMonthYear(text)
			if ok {
				return month, year, nil
			}
			// Also check individual cells
			for _, cell := range row.Cells {
				cellText := cell.GetText()
				month, year, ok := d.parseMonthYear(cellText)
				if ok {
					return month, year, nil
				}
			}
		}
	}

	return 0, 0, fmt.Errorf("could not find month and year in document")
}

// parseMonthYear parses month and year from text
func (d *TableDetector) parseMonthYear(text string) (time.Month, int, bool) {
	matches := d.monthPattern.FindStringSubmatch(text)
	if len(matches) < 3 {
		return 0, 0, false
	}

	monthName := strings.ToLower(matches[1])
	yearStr := matches[2]

	month := d.russianMonthToNumber(monthName)
	if month == 0 {
		return 0, 0, false
	}

	var year int
	fmt.Sscanf(yearStr, "%d", &year)

	return month, year, true
}

// russianMonthToNumber converts Russian month name to time.Month
func (d *TableDetector) russianMonthToNumber(name string) time.Month {
	months := map[string]time.Month{
		"январь":   time.January,
		"февраль":  time.February,
		"март":     time.March,
		"апрель":   time.April,
		"май":      time.May,
		"июнь":     time.June,
		"июль":     time.July,
		"август":   time.August,
		"сентябрь": time.September,
		"октябрь":  time.October,
		"ноябрь":   time.November,
		"декабрь":  time.December,
	}

	return months[strings.ToLower(name)]
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetFormatName returns human-readable format name
func GetFormatName(format TableFormat) string {
	switch format {
	case FormatTimeSlots:
		return "Time Slots Format"
	case FormatDesignation:
		return "У/В Designation Format"
	case FormatCalendarGrid:
		return "Calendar Grid Format"
	default:
		return "Unknown Format"
	}
}
