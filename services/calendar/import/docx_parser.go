package importschedule

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/agnivade/levenshtein"

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
	// Open document using our custom DOCX reader
	doc, err := ReadDocx(content)
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
func (p *ScheduleParser) parseTimeSlotsFormat(doc *DocxDocument, result *ParsedSchedule) error {
	if len(doc.Tables) == 0 {
		return fmt.Errorf("no tables found")
	}

	table := doc.Tables[0]
	rows := table.Rows
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
		cells := row.Cells

		if len(cells) < 2 {
			continue
		}

		// First column: date
		dateText := strings.TrimSpace(cells[0].GetText())
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
			cellText := strings.TrimSpace(cells[colIdx].GetText())
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
func (p *ScheduleParser) parseDesignationFormat(doc *DocxDocument, result *ParsedSchedule) error {
	if len(doc.Tables) == 0 {
		return fmt.Errorf("no tables found")
	}

	table := doc.Tables[0]
	rows := table.Rows
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
		cells := row.Cells

		if len(cells) < 2 {
			continue
		}

		// First column: name
		userName := strings.TrimSpace(cells[0].GetText())
		if userName == "" {
			continue
		}

		// Skip if first column looks like a date (table might be transposed)
		if !p.looksLikeName(userName) {
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
			cellText := strings.ToUpper(strings.TrimSpace(cells[colIdx].GetText()))
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
func (p *ScheduleParser) parseCalendarGridFormat(doc *DocxDocument, result *ParsedSchedule) error {
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
func (p *ScheduleParser) parseTimeSlots(headerRow DocxRow) ([]*TimeSlot, error) {
	cells := headerRow.Cells
	slots := make([]*TimeSlot, 0)

	// Patterns for time slots:
	// 1. "10:00-14:00" or "10.00-14.00"
	timePattern1 := regexp.MustCompile(`(\d{1,2})[:.](\d{2})\s*[-–—]\s*(\d{1,2})[:.](\d{2})`)
	// 2. "С 9 до 14 часов" or "с 9 до 14"
	timePattern2 := regexp.MustCompile(`(?i)[сc]\s*(\d{1,2})\s*(?:[:.](\d{2}))?\s*до\s*(\d{1,2})\s*(?:[:.](\d{2}))?`)
	// 3. "9:00 - 14:00" with spaces
	timePattern3 := regexp.MustCompile(`(\d{1,2})\s*[-–—]\s*(\d{1,2})`)

	for i := 1; i < len(cells); i++ { // Skip first column (dates)
		cellText := cells[i].GetText()
		var slot *TimeSlot

		// Try pattern 1: "10:00-14:00"
		if matches := timePattern1.FindStringSubmatch(cellText); len(matches) >= 5 {
			var startH, startM, endH, endM int
			fmt.Sscanf(matches[1], "%d", &startH)
			fmt.Sscanf(matches[2], "%d", &startM)
			fmt.Sscanf(matches[3], "%d", &endH)
			fmt.Sscanf(matches[4], "%d", &endM)
			slot = &TimeSlot{
				Start: fmt.Sprintf("%02d:%02d", startH, startM),
				End:   fmt.Sprintf("%02d:%02d", endH, endM),
			}
		}

		// Try pattern 2: "С 9 до 14 часов"
		if slot == nil {
			if matches := timePattern2.FindStringSubmatch(cellText); len(matches) >= 4 {
				var startH, endH int
				var startM, endM int
				fmt.Sscanf(matches[1], "%d", &startH)
				fmt.Sscanf(matches[3], "%d", &endH)
				if len(matches) > 2 && matches[2] != "" {
					fmt.Sscanf(matches[2], "%d", &startM)
				}
				if len(matches) > 4 && matches[4] != "" {
					fmt.Sscanf(matches[4], "%d", &endM)
				}
				slot = &TimeSlot{
					Start: fmt.Sprintf("%02d:%02d", startH, startM),
					End:   fmt.Sprintf("%02d:%02d", endH, endM),
				}
			}
		}

		// Try pattern 3: simple "9-14" (hours only)
		if slot == nil {
			if matches := timePattern3.FindStringSubmatch(cellText); len(matches) >= 3 {
				var startH, endH int
				fmt.Sscanf(matches[1], "%d", &startH)
				fmt.Sscanf(matches[2], "%d", &endH)
				slot = &TimeSlot{
					Start: fmt.Sprintf("%02d:00", startH),
					End:   fmt.Sprintf("%02d:00", endH),
				}
			}
		}

		if slot != nil {
			slots = append(slots, slot)
		}
	}

	if len(slots) == 0 {
		return nil, fmt.Errorf("no time slots found in header")
	}

	return slots, nil
}

// parseDateHeader parses dates from header row (Format 2/3)
func (p *ScheduleParser) parseDateHeader(headerRow DocxRow, month time.Month, year int) ([]time.Time, error) {
	cells := headerRow.Cells
	dates := make([]time.Time, 0)

	for i := 1; i < len(cells); i++ { // Skip first column (names)
		cellText := strings.TrimSpace(cells[i].GetText())

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
	text = strings.TrimSpace(text)

	// Try to parse full date format "DD.MM.YYYY"
	fullDatePattern := regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})$`)
	if matches := fullDatePattern.FindStringSubmatch(text); len(matches) >= 4 {
		var day, mon, yr int
		fmt.Sscanf(matches[1], "%d", &day)
		fmt.Sscanf(matches[2], "%d", &mon)
		fmt.Sscanf(matches[3], "%d", &yr)
		if day >= 1 && day <= 31 && mon >= 1 && mon <= 12 {
			return time.Date(yr, time.Month(mon), day, 0, 0, 0, 0, time.Local), true
		}
	}

	// Try to parse "DD.MM" format
	shortDatePattern := regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})$`)
	if matches := shortDatePattern.FindStringSubmatch(text); len(matches) >= 3 {
		var day, mon int
		fmt.Sscanf(matches[1], "%d", &day)
		fmt.Sscanf(matches[2], "%d", &mon)
		if day >= 1 && day <= 31 && mon >= 1 && mon <= 12 {
			return time.Date(year, time.Month(mon), day, 0, 0, 0, 0, time.Local), true
		}
	}

	// Try to extract just day number
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
		if name != "" && !p.isIgnoredText(name) && p.looksLikeName(name) {
			names = append(names, name)
		}
	}

	return names
}

// looksLikeName checks if text looks like a person's name (not a date or number)
func (p *ScheduleParser) looksLikeName(text string) bool {
	// Skip if it's just a number
	if matched, _ := regexp.MatchString(`^\d+$`, text); matched {
		return false
	}

	// Skip if it looks like a date (DD.MM.YYYY or DD.MM or DD/MM/YYYY etc)
	datePatterns := []string{
		`^\d{1,2}\.\d{1,2}\.\d{2,4}$`,  // 19.01.2026
		`^\d{1,2}\.\d{1,2}$`,            // 19.01
		`^\d{1,2}/\d{1,2}/\d{2,4}$`,     // 19/01/2026
		`^\d{1,2}-\d{1,2}-\d{2,4}$`,     // 19-01-2026
		`^\d{1,2}\s+(января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)`,
	}

	textLower := strings.ToLower(text)
	for _, pattern := range datePatterns {
		if matched, _ := regexp.MatchString(pattern, textLower); matched {
			return false
		}
	}

	// A name should contain at least one letter
	if matched, _ := regexp.MatchString(`[a-zA-Zа-яА-ЯёЁ]`, text); !matched {
		return false
	}

	// Skip very short strings (likely not names)
	if len([]rune(text)) < 2 {
		return false
	}

	return true
}

// isIgnoredText checks if text should be ignored (not a name)
func (p *ScheduleParser) isIgnoredText(text string) bool {
	ignored := []string{"у", "в", "-", "–", "—", "пн", "вт", "ср", "чт", "пт", "сб", "вс"}
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
	// Parse start and end hours
	var startHour, endHour int
	fmt.Sscanf(startTime, "%d:", &startHour)
	fmt.Sscanf(endTime, "%d:", &endHour)

	// Morning shift: starts early (before 12:00), ends before or around 15:00
	// Examples: 09:00-14:00, 10:00-14:00, 08:00-13:00
	if startHour >= 7 && startHour <= 11 && endHour >= 12 && endHour <= 15 {
		return models.ShiftMorning
	}

	// Evening shift: starts around midday (12:00-15:00), ends in evening (17:00-21:00)
	// Examples: 14:00-18:00, 14:00-19:00, 13:00-18:00
	if startHour >= 12 && startHour <= 15 && endHour >= 17 && endHour <= 21 {
		return models.ShiftEvening
	}

	// Full day: starts early, ends late (8+ hours)
	// Examples: 09:00-18:00, 10:00-19:00
	if startHour >= 7 && startHour <= 11 && endHour >= 17 && endHour <= 21 {
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
	nameParts := strings.Fields(nameNorm)

	for _, user := range users {
		userName := p.normalizeName(user.Name)
		userParts := strings.Fields(userName)

		// Method 1: Full string similarity
		score := p.calculateSimilarity(nameNorm, userName)

		// Method 2: Check if document name is contained in user name (surname match)
		// e.g., "Козлов" matches "Козлов Иван Петрович"
		if score < MinMatchScore {
			for _, userPart := range userParts {
				partScore := p.calculateSimilarity(nameNorm, userPart)
				if partScore > score {
					score = partScore
				}
			}
		}

		// Method 3: Check if user name part is contained in document name
		// e.g., "Козлов И.П." matches "Козлов"
		if score < MinMatchScore {
			for _, namePart := range nameParts {
				for _, userPart := range userParts {
					partScore := p.calculateSimilarity(namePart, userPart)
					if partScore > score {
						score = partScore
					}
				}
			}
		}

		// Method 4: Check if name starts with same letters (handles initials)
		// e.g., "Козлов" matches "Козлов И." or "К. Иванов"
		if score < MinMatchScore && len(userParts) > 0 && len(nameParts) > 0 {
			// Compare first parts (usually surname)
			if strings.HasPrefix(userParts[0], nameParts[0]) || strings.HasPrefix(nameParts[0], userParts[0]) {
				score = 0.8 // High confidence for prefix match
			}
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
