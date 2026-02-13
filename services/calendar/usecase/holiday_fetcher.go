package usecase

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/logger"
)

const (
	xmlCalendarBaseURL = "https://raw.githubusercontent.com/xmlcalendar/data/master/ru/%d/calendar.xml"
	httpTimeout        = 10 * time.Second
)

// FetchHolidaysFromXMLCalendar fetches and parses holiday data for a given year
func FetchHolidaysFromXMLCalendar(year int) ([]models.Holiday, error) {
	url := fmt.Sprintf(xmlCalendarBaseURL, year)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar XML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xmlcalendar returned status %d for year %d", resp.StatusCode, year)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var cal models.XMLCalendar
	if err := xml.Unmarshal(body, &cal); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	return transformXMLToHolidays(cal, year), nil
}

// transformXMLToHolidays converts parsed XML calendar into Holiday slice
func transformXMLToHolidays(cal models.XMLCalendar, year int) []models.Holiday {
	// Build holiday title lookup: id -> title
	titleMap := make(map[string]string)
	for _, h := range cal.Holidays.Items {
		titleMap[h.ID] = h.Title
	}

	holidays := make([]models.Holiday, 0, len(cal.Days.Items))

	for _, day := range cal.Days.Items {
		if len(day.Date) != 5 {
			logger.WithFields(map[string]interface{}{
				"day":  day.Date,
				"year": year,
			}).Warn("Unexpected date format in xmlcalendar data")
			continue
		}

		// Convert MM.DD to YYYY-MM-DD
		date := fmt.Sprintf("%d-%s-%s", year, day.Date[0:2], day.Date[3:5])

		var holidayType models.HolidayType
		var name string

		switch day.Type {
		case "1":
			holidayType = models.HolidayTypeHoliday
			name = titleMap[day.HolidayID]
			if name == "" {
				name = "Праздничный день"
			}
		case "2":
			holidayType = models.HolidayTypeShortened
			name = "Сокращённый день"
			if title := titleMap[day.HolidayID]; title != "" {
				name = fmt.Sprintf("Сокращённый день (%s)", title)
			}
		case "3":
			holidayType = models.HolidayTypeMovedWeekend
			name = "Рабочий день (перенос)"
			if day.From != "" {
				name = fmt.Sprintf("Рабочий день (за %s)", day.From)
			}
		default:
			logger.WithFields(map[string]interface{}{
				"day":  day.Date,
				"type": day.Type,
				"year": year,
			}).Warn("Unknown day type in xmlcalendar data")
			continue
		}

		holidays = append(holidays, models.Holiday{
			Date: date,
			Name: name,
			Type: holidayType,
		})
	}

	return holidays
}
