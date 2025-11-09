package seeders

import (
	"fmt"
	"time"

	calendarModels "tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/database"
)

// SeedCalendar creates calendar events with participants and reminders
func SeedCalendar(db *database.DB, eventCount int) ([]*calendarModels.Event, error) {
	users := GetUsers()
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found, please seed users first")
	}

	var events []*calendarModels.Event

	eventTemplates := []struct {
		Title       string
		Description string
		Type        calendarModels.EventType
		Duration    time.Duration
	}{
		{
			Title:       "Ежедневный стендап",
			Description: "Обсуждение прогресса и планов на день",
			Type:        calendarModels.EventTypeMeeting,
			Duration:    15 * time.Minute,
		},
		{
			Title:       "Спринт планирование",
			Description: "Планирование задач на следующий спринт",
			Type:        calendarModels.EventTypeMeeting,
			Duration:    2 * time.Hour,
		},
		{
			Title:       "Ретроспектива спринта",
			Description: "Обсуждение результатов спринта",
			Type:        calendarModels.EventTypeMeeting,
			Duration:    1 * time.Hour,
		},
		{
			Title:       "Код-ревью сессия",
			Description: "Групповое ревью важных изменений",
			Type:        calendarModels.EventTypeMeeting,
			Duration:    1 * time.Hour,
		},
		{
			Title:       "Встреча с клиентом",
			Description: "Демонстрация новых функций",
			Type:        calendarModels.EventTypeMeeting,
			Duration:    1 * time.Hour,
		},
		{
			Title:       "Дедлайн: Релиз версии 2.0",
			Description: "Финальная дата релиза",
			Type:        calendarModels.EventTypeDeadline,
			Duration:    0,
		},
		{
			Title:       "Дедлайн: Завершение миграции БД",
			Description: "Миграция должна быть завершена",
			Type:        calendarModels.EventTypeDeadline,
			Duration:    0,
		},
		{
			Title:       "День рождения коллеги",
			Description: "Празднование в офисе",
			Type:        calendarModels.EventTypePersonal,
			Duration:    30 * time.Minute,
		},
		{
			Title:       "Обучение: Новые технологии",
			Description: "Семинар по новым инструментам разработки",
			Type:        calendarModels.EventTypeMeeting,
			Duration:    3 * time.Hour,
		},
		{
			Title:       "Корпоративное мероприятие",
			Description: "Тимбилдинг для всей команды",
			Type:        calendarModels.EventTypePersonal,
			Duration:    4 * time.Hour,
		},
	}

	colors := []string{"#FF6B6B", "#4ECDC4", "#45B7D1", "#FFA07A", "#98D8C8", "#F7DC6F", "#BB8FCE", "#85C1E2"}

	for i := 0; i < eventCount; i++ {
		template := eventTemplates[randInt(0, len(eventTemplates)-1)]
		creator := GetRandomUser()

		// Random time in next 60 days or past 30 days
		daysOffset := randInt(-30, 60)
		startTime := time.Now().Add(time.Duration(daysOffset) * 24 * time.Hour)

		// Random hour between 9-18
		hour := randInt(9, 18)
		minute := []int{0, 15, 30, 45}[randInt(0, 3)]
		startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), hour, minute, 0, 0, startTime.Location())

		endTime := startTime.Add(template.Duration)

		// 10% chance of all-day event
		isAllDay := randInt(1, 10) == 1
		if isAllDay {
			startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
			endTime = startTime.Add(24 * time.Hour)
		}

		event := &calendarModels.Event{
			Title:       template.Title,
			Description: template.Description,
			StartTime:   startTime,
			EndTime:     endTime,
			AllDay:      isAllDay,
			Type:        template.Type,
			CreatedBy:   creator.ID,
			Color:       colors[randInt(0, len(colors)-1)],
			IsPrivate:   randInt(1, 5) == 1, // 20% private events
		}

		// 20% chance of having location
		if randInt(1, 5) == 1 {
			locations := []string{
				"Конференц-зал A",
				"Конференц-зал B",
				"Переговорная 1",
				"Переговорная 2",
				"Zoom",
				"Google Meet",
				"Офис, 3 этаж",
			}
			event.Location = locations[randInt(0, len(locations)-1)]
		}

		// 15% chance of recurring event
		if randInt(1, 100) <= 15 {
			event.IsRecurring = true
			recurrenceRules := []string{
				`{"frequency":"daily","interval":1}`,
				`{"frequency":"weekly","interval":1,"by_day":["MO","WE","FR"]}`,
				`{"frequency":"weekly","interval":1}`,
				`{"frequency":"monthly","interval":1}`,
			}
			event.RecurrenceRule = recurrenceRules[randInt(0, len(recurrenceRules)-1)]
		}

		if err := db.DB.Create(event).Error; err != nil {
			return nil, fmt.Errorf("failed to create event: %w", err)
		}

		// Add participants for meetings (not for personal events or deadlines)
		if template.Type == calendarModels.EventTypeMeeting {
			participantCount := randInt(2, 8)
			participants := GetRandomUsers(participantCount)

			// Ensure creator is in participants
			creatorFound := false
			for _, p := range participants {
				if p.ID == creator.ID {
					creatorFound = true
					break
				}
			}
			if !creatorFound {
				participants = append(participants, creator)
			}

			for _, participant := range participants {
				// 70% accepted, 10% declined, 10% maybe, 10% pending
				var status calendarModels.ParticipantStatus
				roll := randInt(1, 100)
				switch {
				case roll <= 70:
					status = calendarModels.ParticipantStatusAccepted
				case roll <= 80:
					status = calendarModels.ParticipantStatusDeclined
				case roll <= 90:
					status = calendarModels.ParticipantStatusMaybe
				default:
					status = calendarModels.ParticipantStatusPending
				}

				eventParticipant := &calendarModels.EventParticipant{
					EventID: event.ID,
					UserID:  participant.ID,
					Status:  status,
				}

				// Add response time if not pending
				if status != calendarModels.ParticipantStatusPending {
					responseTime := startTime.Add(-time.Duration(randInt(1, 48)) * time.Hour)
					eventParticipant.RespondedAt = &responseTime
				}

				if err := db.DB.Create(eventParticipant).Error; err != nil {
					return nil, fmt.Errorf("failed to create event participant: %w", err)
				}
			}
		}

		events = append(events, event)
	}

	return events, nil
}
