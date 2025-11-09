package seeders

import (
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	taskModels "tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"
)

// SeedTasks creates tasks with various statuses, subtasks, comments, and checklists
func SeedTasks(db *database.DB, taskCount int) ([]*taskModels.Task, error) {
	users := GetUsers()
	if len(users) == 0 {
		return nil, fmt.Errorf("no users found, please seed users first")
	}

	var tasks []*taskModels.Task

	taskTitles := []string{
		"Разработать новый функционал аутентификации",
		"Исправить баги в модуле отчетов",
		"Обновить документацию API",
		"Провести код-ревью pull request",
		"Оптимизировать запросы к базе данных",
		"Настроить CI/CD пайплайн",
		"Написать unit-тесты для сервиса пользователей",
		"Провести встречу с клиентом",
		"Подготовить презентацию для руководства",
		"Исследовать новые технологии для проекта",
		"Рефакторинг legacy кода",
		"Добавить поддержку мультиязычности",
		"Интеграция с внешним API",
		"Настройка мониторинга и алертов",
		"Подготовка к релизу версии 2.0",
		"Миграция данных в новую структуру",
		"Улучшение производительности фронтенда",
		"Создание дизайн-макетов",
		"Анализ метрик и статистики",
		"Планирование следующего спринта",
	}

	statuses := []taskModels.TaskStatus{
		taskModels.TaskStatusNew,
		taskModels.TaskStatusViewed,
		taskModels.TaskStatusInProgress,
		taskModels.TaskStatusReview,
		taskModels.TaskStatusDone,
		taskModels.TaskStatusCancelled,
	}

	priorities := []taskModels.TaskPriority{
		taskModels.TaskPriorityLow,
		taskModels.TaskPriorityMedium,
		taskModels.TaskPriorityHigh,
		taskModels.TaskPriorityCritical,
	}

	// Status distribution: 10% new, 20% viewed, 30% in_progress, 15% review, 20% done, 5% cancelled
	statusWeights := map[taskModels.TaskStatus]int{
		taskModels.TaskStatusNew:        10,
		taskModels.TaskStatusViewed:     20,
		taskModels.TaskStatusInProgress: 30,
		taskModels.TaskStatusReview:     15,
		taskModels.TaskStatusDone:       20,
		taskModels.TaskStatusCancelled:  5,
	}

	// Priority distribution: 40% low, 35% medium, 20% high, 5% critical
	priorityWeights := map[taskModels.TaskPriority]int{
		taskModels.TaskPriorityLow:      40,
		taskModels.TaskPriorityMedium:   35,
		taskModels.TaskPriorityHigh:     20,
		taskModels.TaskPriorityCritical: 5,
	}

	for i := 0; i < taskCount; i++ {
		creator := GetRandomUser()
		assignee := GetRandomUser()

		status := getWeightedStatus(statusWeights)
		priority := getWeightedPriority(priorityWeights)

		task := &taskModels.Task{
			Title:             taskTitles[randInt(0, len(taskTitles)-1)],
			Description:       gofakeit.Paragraph(2, 5, 10, " "),
			Status:            status,
			Priority:          priority,
			CreatedByUserID:   creator.ID,
			AssignedToUserID:  &assignee.ID,
			ProgressPercentage: getProgressByStatus(status),
		}

		// Set due date (70% of tasks have due date)
		if randInt(1, 10) <= 7 {
			dueDate := time.Now().Add(time.Duration(randInt(-30, 60)) * 24 * time.Hour)
			task.DueDate = &dueDate
		}

		// Set first viewed time for non-new tasks
		if status != taskModels.TaskStatusNew {
			viewedAt := time.Now().Add(-time.Duration(randInt(1, 30)) * 24 * time.Hour)
			task.FirstViewedAt = &viewedAt
		}

		// Set completed time for done tasks
		if status == taskModels.TaskStatusDone {
			completedAt := time.Now().Add(-time.Duration(randInt(1, 10)) * 24 * time.Hour)
			task.CompletedAt = &completedAt
			task.ProgressPercentage = 100
		}

		// 20% chance to have department assignment instead of user
		if randInt(1, 5) == 1 {
			dept := GetRandomDepartment()
			task.AssignedToDepartment = &dept.ID
			task.AssignedToUserID = nil
		}

		if err := db.DB.Create(task).Error; err != nil {
			return nil, fmt.Errorf("failed to create task: %w", err)
		}

		// Add multiple assignees (30% chance, 2-4 assignees)
		if randInt(1, 10) <= 3 {
			assigneeCount := randInt(2, 4)
			assignees := GetRandomUsers(assigneeCount)
			for _, u := range assignees {
				taskAssignee := &taskModels.TaskAssignee{
					TaskID:           task.ID,
					UserID:           u.ID,
					AssignedByUserID: &creator.ID,
					AssignedAt:       time.Now().Add(-time.Duration(randInt(1, 20)) * 24 * time.Hour),
				}
				db.DB.Create(taskAssignee)
			}
		}

		// Add subtasks (40% chance, 2-5 subtasks)
		if randInt(1, 10) <= 4 {
			subtaskCount := randInt(2, 5)
			for j := 0; j < subtaskCount; j++ {
				subtaskStatus := statuses[randInt(0, len(statuses)-1)]
				subtask := &taskModels.Task{
					Title:              fmt.Sprintf("Подзадача %d: %s", j+1, gofakeit.BS()),
					Description:        gofakeit.Sentence(10),
					Status:             subtaskStatus,
					Priority:           priorities[randInt(0, len(priorities)-1)],
					ParentTaskID:       &task.ID,
					CreatedByUserID:    creator.ID,
					AssignedToUserID:   &assignee.ID,
					ProgressPercentage: getProgressByStatus(subtaskStatus),
				}
				db.DB.Create(subtask)
			}
		}

		// Add comments (60% chance, 1-8 comments)
		if randInt(1, 10) <= 6 {
			commentCount := randInt(1, 8)
			for j := 0; j < commentCount; j++ {
				commenter := GetRandomUser()
				comment := &taskModels.TaskComment{
					TaskID:  task.ID,
					UserID:  commenter.ID,
					Content: gofakeit.Paragraph(1, 2, 5, " "),
				}
				db.DB.Create(comment)
			}
		}

		// Add checklist (30% chance)
		if randInt(1, 10) <= 3 {
			checklist := &taskModels.TaskChecklist{
				TaskID: task.ID,
				Title:  "Список задач",
			}
			if err := db.DB.Create(checklist).Error; err == nil {
				// Add 3-7 checklist items
				itemCount := randInt(3, 7)
				for j := 0; j < itemCount; j++ {
					item := &taskModels.TaskChecklistItem{
						ChecklistID: checklist.ID,
						Title:     gofakeit.BS(),
						IsCompleted: randInt(1, 2) == 1, // 50% chance completed
						Position:       j,
					}
					db.DB.Create(item)
				}
			}
		}

		// Add activity log entries
		activityTypes := []string{"created", "status_changed", "assigned", "commented", "updated"}
		activityCount := randInt(1, 5)
		for j := 0; j < activityCount; j++ {
			activity := &taskModels.TaskActivity{
				TaskID:     task.ID,
				UserID:     GetRandomUser().ID,
				ActionType: activityTypes[randInt(0, len(activityTypes)-1)],
				OldValue:   "Старое значение",
				NewValue:   "Новое значение",
			}
			db.DB.Create(activity)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func getWeightedStatus(weights map[taskModels.TaskStatus]int) taskModels.TaskStatus {
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

	return taskModels.TaskStatusNew
}

func getWeightedPriority(weights map[taskModels.TaskPriority]int) taskModels.TaskPriority {
	total := 0
	for _, weight := range weights {
		total += weight
	}

	r := randInt(1, total)
	current := 0

	for priority, weight := range weights {
		current += weight
		if r <= current {
			return priority
		}
	}

	return taskModels.TaskPriorityMedium
}

func getProgressByStatus(status taskModels.TaskStatus) int {
	switch status {
	case taskModels.TaskStatusNew:
		return 0
	case taskModels.TaskStatusViewed:
		return randInt(0, 10)
	case taskModels.TaskStatusInProgress:
		return randInt(20, 80)
	case taskModels.TaskStatusReview:
		return randInt(80, 95)
	case taskModels.TaskStatusDone:
		return 100
	case taskModels.TaskStatusCancelled:
		return randInt(0, 50)
	default:
		return 0
	}
}
