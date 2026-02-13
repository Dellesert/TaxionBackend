package usecase

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/calendar/models"
	sharedredis "tachyon-messenger/shared/redis"
	"tachyon-messenger/shared/logger"
)

const (
	holidayCacheKeyPrefix = "calendar:holidays:"
	holidayCacheTTL       = 24 * time.Hour
	minYear               = 2020
	maxYear               = 2030
)

// HolidayUsecase defines the interface for holiday business logic
type HolidayUsecase interface {
	GetHolidays(year int) (*models.HolidaysResponse, error)
}

type holidayUsecase struct {
	redisClient *sharedredis.Client
}

// NewHolidayUsecase creates a new holiday usecase
func NewHolidayUsecase(redisClient *sharedredis.Client) HolidayUsecase {
	return &holidayUsecase{
		redisClient: redisClient,
	}
}

// GetHolidays returns holidays for a given year, using Redis cache
func (u *holidayUsecase) GetHolidays(year int) (*models.HolidaysResponse, error) {
	if year < minYear || year > maxYear {
		return nil, fmt.Errorf("validation failed: year must be between %d and %d", minYear, maxYear)
	}

	cacheKey := fmt.Sprintf("%s%d", holidayCacheKeyPrefix, year)

	// Try Redis cache
	cached, err := u.redisClient.Get(cacheKey)
	if err == nil && cached != "" {
		var holidays []models.Holiday
		if err := json.Unmarshal([]byte(cached), &holidays); err == nil {
			logger.WithFields(map[string]interface{}{
				"year":  year,
				"count": len(holidays),
			}).Debug("Holidays served from cache")

			return &models.HolidaysResponse{
				Year:     year,
				Holidays: holidays,
			}, nil
		}
		logger.WithFields(map[string]interface{}{
			"year":  year,
			"error": err.Error(),
		}).Warn("Failed to unmarshal cached holidays, fetching fresh")
	}

	// Fetch from xmlcalendar.ru
	holidays, err := FetchHolidaysFromXMLCalendar(year)
	if err != nil {
		if strings.Contains(err.Error(), "status 404") {
			return nil, fmt.Errorf("holiday data not available for year %d", year)
		}
		return nil, fmt.Errorf("failed to fetch holidays: %w", err)
	}

	// Cache in Redis
	jsonBytes, err := json.Marshal(holidays)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"year":  year,
			"error": err.Error(),
		}).Warn("Failed to marshal holidays for caching")
	} else {
		if cacheErr := u.redisClient.Set(cacheKey, string(jsonBytes), holidayCacheTTL); cacheErr != nil {
			logger.WithFields(map[string]interface{}{
				"year":  year,
				"error": cacheErr.Error(),
			}).Warn("Failed to cache holidays in Redis")
		}
	}

	logger.WithFields(map[string]interface{}{
		"year":  year,
		"count": len(holidays),
	}).Info("Holidays fetched from xmlcalendar.ru and cached")

	return &models.HolidaysResponse{
		Year:     year,
		Holidays: holidays,
	}, nil
}
