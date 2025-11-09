package seeders

import (
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	pollModels "tachyon-messenger/services/poll/models"
	"tachyon-messenger/shared/database"
)

// SeedPolls creates polls with various types, votes, and comments
func SeedPolls(db *database.DB, pollCount int) ([]*pollModels.Poll, error) {
	users := GetUsers()
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found, please seed users first")
	}

	departments := GetDepartments()
	var polls []*pollModels.Poll

	pollTemplates := []struct {
		Title       string
		Description string
		Type        pollModels.PollType
		Options     []string
	}{
		{
			Title:       "Какой язык программирования использовать для нового проекта?",
			Description: "Нужно выбрать основной язык для разработки микросервисов",
			Type:        pollModels.PollTypeSingleChoice,
			Options:     []string{"Go", "Python", "Java", "Node.js", "Rust"},
		},
		{
			Title:       "Какие функции нужно добавить в следующем релизе?",
			Description: "Выберите наиболее важные функции",
			Type:        pollModels.PollTypeMultipleChoice,
			Options:     []string{"Темная тема", "Экспорт в PDF", "Интеграция с Slack", "Мобильное приложение", "API v2"},
		},
		{
			Title:       "Удобно ли вам время ежедневных стендапов?",
			Description: "Текущее время - 10:00",
			Type:        pollModels.PollTypeSingleChoice,
			Options:     []string{"Да, удобно", "Нет, слишком рано", "Нет, слишком поздно", "Предпочитаю асинхронные обновления"},
		},
		{
			Title:       "Какие инструменты для коммуникации вы используете?",
			Description: "Можно выбрать несколько",
			Type:        pollModels.PollTypeMultipleChoice,
			Options:     []string{"Slack", "Teams", "Telegram", "Email", "Zoom", "Google Meet"},
		},
	}

	// Status distribution: 60% active, 40% closed
	statusWeights := map[pollModels.PollStatus]int{
		pollModels.PollStatusActive: 60,
		pollModels.PollStatusClosed: 40,
	}

	for i := 0; i < pollCount; i++ {
		template := pollTemplates[randInt(0, len(pollTemplates)-1)]
		creator := GetRandomUser()
		status := getWeightedPollStatus(statusWeights)

		// Time setup
		startTime := time.Now().Add(-time.Duration(randInt(1, 30)) * 24 * time.Hour)
		endTime := startTime.Add(time.Duration(randInt(3, 14)) * 24 * time.Hour)

		poll := &pollModels.Poll{
			Title:            template.Title,
			Description:      template.Description,
			Type:             template.Type,
			Status:           status,
			Visibility:       getRandomVisibility(),
			CreatedBy:        creator.ID,
			StartTime:        &startTime,
			EndTime:          &endTime,
			AllowAnonymous:   randInt(1, 3) == 1,      // 33% allow anonymous
			AllowMultipleVote: randInt(1, 5) == 1,     // 20% allow multiple votes
			RequireComment:   randInt(1, 10) == 1,     // 10% require comment
			ShowResults:      true,
			ShowResultsAfter: true,
		}

		// 30% chance to be department-specific
		if randInt(1, 10) <= 3 && len(departments) > 0 {
			dept := departments[randInt(0, len(departments)-1)]
			poll.DepartmentID = &dept.ID
			poll.Visibility = pollModels.PollVisibilityDepartment
		}

		// Random category
		categories := []string{"Технические", "HR", "Общие", "Продукт", "Процессы"}
		poll.Category = categories[randInt(0, len(categories)-1)]

		if err := db.DB.Create(poll).Error; err != nil {
			return nil, fmt.Errorf("failed to create poll: %w", err)
		}

		// Create options
		for j, optionText := range template.Options {
			option := &pollModels.PollOption{
				PollID:   poll.ID,
				Text:     optionText,
				Position: j,
			}
			if err := db.DB.Create(option).Error; err != nil {
				return nil, fmt.Errorf("failed to create poll option: %w", err)
			}
		}

		// Add votes
		var options []pollModels.PollOption
		db.DB.Where("poll_id = ?", poll.ID).Find(&options)

		// 40-90% of users vote
		voterCount := randInt(len(users)*40/100, len(users)*90/100)
		voters := GetRandomUsers(voterCount)

		for _, voter := range voters {
			// Create votes based on poll type
			switch template.Type {
			case pollModels.PollTypeSingleChoice:
				// Vote for one option
				if len(options) > 0 {
					option := options[randInt(0, len(options)-1)]
					vote := &pollModels.PollVote{
						PollID:   poll.ID,
						UserID:   &voter.ID,
						OptionID: &option.ID,
					}
					db.DB.Create(vote)
				}

			case pollModels.PollTypeMultipleChoice:
				// Vote for 1-3 options
				voteCount := randInt(1, min(3, len(options)))
				shuffled := make([]pollModels.PollOption, len(options))
				copy(shuffled, options)
				for k := len(shuffled) - 1; k > 0; k-- {
					j := randInt(0, k)
					shuffled[k], shuffled[j] = shuffled[j], shuffled[k]
				}

				for k := 0; k < voteCount; k++ {
					vote := &pollModels.PollVote{
						PollID:   poll.ID,
						UserID:   &voter.ID,
						OptionID: &shuffled[k].ID,
					}
					db.DB.Create(vote)
				}
			}
		}

		// Add comments (40% chance, 1-5 comments)
		if randInt(1, 10) <= 4 {
			commentCount := randInt(1, 5)
			for j := 0; j < commentCount; j++ {
				commenter := GetRandomUser()
				comment := &pollModels.PollComment{
					PollID:  poll.ID,
					UserID:  commenter.ID,
					Content: gofakeit.Sentence(randInt(5, 15)),
				}
				db.DB.Create(comment)
			}
		}

		polls = append(polls, poll)
	}

	return polls, nil
}

func getWeightedPollStatus(weights map[pollModels.PollStatus]int) pollModels.PollStatus {
	total := 0
	for _, weight := range weights {
		total += weight
	}

	r := randInt(1, total)
	current := 0

	for status, weight := range weights {
		current += weight
		if r <= current {
			return status
		}
	}

	return pollModels.PollStatusActive
}

func getRandomVisibility() pollModels.PollVisibility {
	visibilities := []pollModels.PollVisibility{
		pollModels.PollVisibilityPublic,
		pollModels.PollVisibilityDepartment,
		pollModels.PollVisibilityInviteOnly,
	}
	return visibilities[randInt(0, len(visibilities)-1)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
