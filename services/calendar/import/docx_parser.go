package importschedule

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/agnivade/levenshtein"
	"github.com/unidoc/unioffice/document"

	"tachyon-messenger/services/calendar/models"
	sharedmodels "tachyon-messenger/shared/models"
)

const (
	// MinMatchScore minimum fuzzy match score to consider a match
	MinMatchScore = 0.6
)

// ScheduleParser parses Word documents into schedule entries
type ScheduleParser struct {
	detector    *TableDetector
	timePattern *regexp.Regexp
}

// ParsedSchedule represents parsed schedule data
type ParsedSchedule struct {
	Month    time.Month
	Year     int
	Format   TableFormat
	Entries  []*ParsedEntry
	Users    map[string]*models.ImportedUser // Key: name from doc
	Warnings []string
}

// ParsedEntry represents a single parsed schedule entry
type ParsedEntry struct {
	UserName  string
	Date      time.Time
	StartTime string // "10:00"
	EndTime   string // "14:00"
	ShiftType models.ShiftType
}

// NewScheduleParser creates a new schedule parser
func NewScheduleParser() *ScheduleParser {
	return &ScheduleParser{
		detector:    NewTableDetector(),
		timePattern: regexp.MustCompile(`(\d{1,2})[:\.:](\d{2})\s*-\s*(\d{1,2})[:\.:](\d{2})`),
	}
}

// ParseDocument parses a Word document into schedule data
func (p *ScheduleParser) ParseDocument(content []byte) (*ParsedSchedule, error) {
	// Open document
	doc, err := document.Read(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to read document: %w", err)
	}

	// Detect format
	format, err := p.detector.DetectFormat(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to detect format: %w", err)
	}

	// Extract month and year
	month, year, err := p.detector.ExtractMonthYear(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract month/year: %w", err)
	}

	result := &ParsedSchedule{
		Month:    month,
		Year:     year,
		Format:   format,
		Entries:  make([]*ParsedEntry, 0),
		Users:    make(map[string]*models.ImportedUser),
		Warnings: make([]string, 0),
	}

	// Parse based on format
	switch format {
	case FormatTimeSlots:
		err = p.parseTimeSlotsFormat(doc, result)
	case FormatDesignation:
		err = p.parseDesignationFormat(doc, result)
	case FormatCalendarGrid:
		err = p.parseCalendarGridFormat(doc, result)
	default:
		return nil, fmt.Errorf("unsupported format: %s", GetFormatName(format))
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

// parseTimeSlotsFormat parses Format 1: Time slots with names
func (p *ScheduleParser) parseTimeSlotsFormat(doc *document.Document, result *ParsedSchedule) error {
	tables := doc.Tables()
	if len(tables) == 0 {
		return fmt.Errorf("no tables found")
	}

	table := tables[0]
	rows := table.Rows()
	if len(rows) < 2 {
		return fmt.Errorf("insufficient rows in table")
	}

	// Parse header to get time slots
	headerRow := rows[0]
	timeSlots, err := p.parseTimeSlots(headerRow)
	if err != nil {
		return fmt.Errorf("failed to parse time slots: %w", err)
	}

	// Parse data rows
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		cells := row.Cells()

		if len(cells) < 2 {
			continue
		}

		// First column: date
		dateText := strings.TrimSpace(p.detector.extractCellText(cells[0]))
		if dateText == "" {
			continue
		}

		date, ok := p.parseDate(dateText, result.Month, result.Year)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Invalid date: %s", dateText))
			continue
		}

		// Process each time slot
		for colIdx := 1; colIdx < len(cells) && colIdx <= len(timeSlots); colIdx++ {
			cellText := strings.TrimSpace(p.detector.extractCellText(cells[colIdx]))
			if cellText == "" {
				continue
			}

			// Extract names from cell (can be multiple names separated by comma)
			names := p.extractNames(cellText)
			timeSlot := timeSlots[colIdx-1]

			for _, name := range names {
				entry := &ParsedEntry{
					UserName:  name,
					Date:      date,
					StartTime: timeSlot.Start,
					EndTime:   timeSlot.End,
					ShiftType: p.determineShiftType(timeSlot.Start, timeSlot.End),
				}

				result.Entries = append(result.Entries, entry)

				// Track user
				if _, exists := result.Users[name]; !exists {
					result.Users[name] = &models.ImportedUser{
						Name:        name,
						IsUnmatched: true,
					}
				}
			}
		}
	}

	return nil
}

// parseDesignationFormat parses Format 2: У/В designation
func (p *ScheduleParser) parseDesignationFormat(doc *document.Document, result *ParsedSchedule) error {
	tables := doc.Tables()
	if len(tables) == 0 {
		return fmt.Errorf("no tables found")
	}

	table := tables[0]
	rows := table.Rows()
	if len(rows) < 2 {
		return fmt.Errorf("insufficient rows in table")
	}

	// Parse header to get dates
	headerRow := rows[0]
	dates, err := p.parseDateHeader(headerRow, result.Month, result.Year)
	if err != nil {
		return fmt.Errorf("failed to parse date header: %w", err)
	}

	// Parse data rows
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		cells := row.Cells()

		if len(cells) < 2 {
			continue
		}

		// First column: name
		userName := strings.TrimSpace(p.detector.extractCellText(cells[0]))
		if userName == "" {
			continue
		}

		// Track user
		if _, exists := result.Users[userName]; !exists {
			result.Users[userName] = &models.ImportedUser{
				Name:        userName,
				IsUnmatched: true,
			}
		}

		// Process each date column
		for colIdx := 1; colIdx < len(cells) && colIdx <= len(dates); colIdx++ {
			cellText := strings.ToUpper(strings.TrimSpace(p.detector.extractCellText(cells[colIdx])))
			if cellText == "" {
				continue
			}

			date := dates[colIdx-1]

			// Parse У (morning) and/or В (evening)
			hasU := strings.Contains(cellText, "У")
			hasV := strings.Contains(cellText, "В")

			if hasU {
				entry := &ParsedEntry{
					UserName:  userName,
					Date:      date,
					StartTime: "10:00",
					EndTime:   "14:00",
					ShiftType: models.ShiftMorning,
				}
				result.Entries = append(result.Entries, entry)
			}

			if hasV {
				entry := &ParsedEntry{
					UserName:  userName,
					Date:      date,
					StartTime: "14:00",
					EndTime:   "18:00",
					ShiftType: models.ShiftEvening,
				}
				result.Entries = append(result.Entries, entry)
			}
		}
	}

	return nil
}

// parseCalendarGridFormat parses Format 3: Calendar grid
func (p *ScheduleParser) parseCalendarGridFormat(doc *document.Document, result *ParsedSchedule) error {
	// Similar to parseDesignationFormat but with different interpretation
	// For now, treat it as designation format
	return p.parseDesignationFormat(doc, result)
}

// TimeSlot represents a time slot column
type TimeSlot struct {
	Start string // "10:00"
	End   string // "14:00"
}

// parseTimeSlots parses time slots from header row
func (p *ScheduleParser) parseTimeSlots(headerRow document.Row) ([]*TimeSlot, error) {
	cells := headerRow.Cells()
	slots := make([]*TimeSlot, 0)

	for i := 1; i < len(cells); i++ { // Skip first column (dates)
		cellText := p.detector.extractCellText(cells[i])
		matches := p.timePattern.FindStringSubmatch(cellText)

		if len(matches) >= 5 {
			slot := &TimeSlot{
				Start: fmt.Sprintf("%02s:%02s", matches[1], matches[2]),
				End:   fmt.Sprintf("%02s:%02s", matches[3], matches[4]),
			}
			slots = append(slots, slot)
		}
	}

	if len(slots) == 0 {
		return nil, fmt.Errorf("no time slots found in header")
	}

	return slots, nil
}

// parseDateHeader parses dates from header row (Format 2/3)
func (p *ScheduleParser) parseDateHeader(headerRow document.Row, month time.Month, year int) ([]time.Time, error) {
	cells := headerRow.Cells()
	dates := make([]time.Time, 0)

	for i := 1; i < len(cells); i++ { // Skip first column (names)
		cellText := strings.TrimSpace(p.detector.extractCellText(cells[i]))

		// Try to parse as day number
		var day int
		if n, err := fmt.Sscanf(cellText, "%d", &day); n == 1 && err == nil {
			if day >= 1 && day <= 31 {
				date := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
				dates = append(dates, date)
				continue
			}
		}

		// Could not parse - add warning but continue
		dates = append(dates, time.Time{})
	}

	if len(dates) == 0 {
		return nil, fmt.Errorf("no valid dates found in header")
	}

	return dates, nil
}

// parseDate parses date from text
func (p *ScheduleParser) parseDate(text string, month time.Month, year int) (time.Time, bool) {
	// Try to extract day number
	var day int
	if n, err := fmt.Sscanf(text, "%d", &day); n == 1 && err == nil {
		if day >= 1 && day <= 31 {
			return time.Date(year, month, day, 0, 0, 0, 0, time.Local), true
		}
	}

	return time.Time{}, false
}

// extractNames extracts names from cell text (comma or newline separated)
func (p *ScheduleParser) extractNames(text string) []string {
	// Split by comma or newline
	separators := regexp.MustCompile(`[,\n]+`)
	parts := separators.Split(text, -1)

	names := make([]string, 0)
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name != "" && !p.isIgnoredText(name) {
			names = append(names, name)
		}
	}

	return names
}

// isIgnoredText checks if text should be ignored (not a name)
func (p *ScheduleParser) isIgnoredText(text string) bool {
	ignored := []string{"у", "в", "-", "–", "—"}
	lower := strings.ToLower(text)

	for _, ign := range ignored {
		if lower == ign {
			return true
		}
	}

	return false
}

// determineShiftType determines shift type from times
func (p *ScheduleParser) determineShiftType(startTime, endTime string) models.ShiftType {
	// Standard shifts
	if startTime == "10:00" && endTime == "14:00" {
		return models.ShiftMorning
	}
	if startTime == "14:00" && endTime == "18:00" {
		return models.ShiftEvening
	}
	if startTime == "10:00" && endTime == "18:00" {
		return models.ShiftFullDay
	}

	return models.ShiftCustom
}

// MatchUsers matches parsed user names to actual users using fuzzy matching
func (p *ScheduleParser) MatchUsers(parsed *ParsedSchedule, allUsers []*sharedmodels.User) {
	for name, importedUser := range parsed.Users {
		bestMatch := p.findBestMatch(name, allUsers)
		if bestMatch != nil {
			importedUser.UserID = &bestMatch.ID
			importedUser.MatchScore = bestMatch.Score
			importedUser.IsUnmatched = false
		} else {
			parsed.Warnings = append(parsed.Warnings, fmt.Sprintf("No match found for user: %s", name))
		}
	}
}

// UserMatch represents a user match with score
type UserMatch struct {
	ID    uint
	Score float64
}

// findBestMatch finds best matching user using fuzzy string matching
func (p *ScheduleParser) findBestMatch(name string, users []*sharedmodels.User) *UserMatch {
	var bestMatch *UserMatch
	bestScore := 0.0

	nameNorm := p.normalizeName(name)

	for _, user := range users {
		// Try matching against full name
		fullName := p.normalizeName(fmt.Sprintf("%s %s", user.FirstName, user.LastName))
		score := p.calculateSimilarity(nameNorm, fullName)

		// Also try reversed order
		fullNameReversed := p.normalizeName(fmt.Sprintf("%s %s", user.LastName, user.FirstName))
		scoreReversed := p.calculateSimilarity(nameNorm, fullNameReversed)

		if scoreReversed > score {
			score = scoreReversed
		}

		// Also try just last name
		lastName := p.normalizeName(user.LastName)
		scoreLastName := p.calculateSimilarity(nameNorm, lastName)

		if scoreLastName > score {
			score = scoreLastName
		}

		if score > bestScore && score >= MinMatchScore {
			bestScore = score
			bestMatch = &UserMatch{
				ID:    user.ID,
				Score: score,
			}
		}
	}

	return bestMatch
}

// calculateSimilarity calculates similarity between two strings (0.0 to 1.0)
func (p *ScheduleParser) calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	distance := levenshtein.ComputeDistance(s1, s2)
	maxLen := max(len(s1), len(s2))

	if maxLen == 0 {
		return 0.0
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// normalizeName normalizes name for comparison
func (p *ScheduleParser) normalizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Remove extra whitespace
	name = strings.Join(strings.Fields(name), " ")

	// Remove common prefixes/suffixes
	name = strings.TrimSpace(name)

	return name
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
