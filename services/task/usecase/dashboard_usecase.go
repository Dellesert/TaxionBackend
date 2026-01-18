package usecase

import (
	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
	"tachyon-messenger/shared/logger"
)

// DashboardUsecase defines the interface for dashboard business logic
type DashboardUsecase interface {
	GetDashboard(userID uint, limit int) (*models.DashboardResponse, error)
}

// dashboardUsecase implements DashboardUsecase interface
type dashboardUsecase struct {
	taskRepo       repository.TaskRepository
	userClient     *clients.UserClient
	pollClient     *clients.PollClient
	calendarClient *clients.CalendarClient
}

// NewDashboardUsecase creates a new dashboard usecase
func NewDashboardUsecase(
	taskRepo repository.TaskRepository,
) DashboardUsecase {
	return &dashboardUsecase{
		taskRepo:       taskRepo,
		userClient:     clients.NewUserClient(),
		pollClient:     clients.NewPollClient(),
		calendarClient: clients.NewCalendarClient(),
	}
}

// GetDashboard retrieves dashboard data for a user
func (u *dashboardUsecase) GetDashboard(userID uint, limit int) (*models.DashboardResponse, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	dashboard := &models.DashboardResponse{
		NewTasks:     make([]*models.TaskResponse, 0),
		ActiveTasks:  make([]*models.TaskResponse, 0),
		OverdueTasks: make([]*models.TaskResponse, 0),
		PendingPolls: make([]*models.PendingPollResponse, 0),
		TodayEvents:  make([]*models.TodayEventResponse, 0),
		Counts:       models.DashboardCounts{},
	}

	// Get new tasks
	newTasks, newTasksCount, err := u.taskRepo.GetNewTasksForUser(userID, limit)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Error("Failed to get new tasks for dashboard")
	} else {
		dashboard.NewTasks = u.convertTasksToResponses(newTasks, userID)
		dashboard.Counts.NewTasksCount = newTasksCount
	}

	// Get active tasks
	activeTasks, activeTasksCount, err := u.taskRepo.GetActiveTasksForUser(userID, limit)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Error("Failed to get active tasks for dashboard")
	} else {
		dashboard.ActiveTasks = u.convertTasksToResponses(activeTasks, userID)
		dashboard.Counts.ActiveTasksCount = activeTasksCount
	}

	// Get overdue tasks
	overdueTasks, overdueTasksCount, err := u.taskRepo.GetOverdueTasksForUser(userID, limit)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Error("Failed to get overdue tasks for dashboard")
	} else {
		dashboard.OverdueTasks = u.convertTasksToResponses(overdueTasks, userID)
		dashboard.Counts.OverdueTasksCount = overdueTasksCount
	}

	// Get pending polls from poll service
	pendingPolls, pendingPollsCount, err := u.pollClient.GetPendingPollsForUser(userID, limit)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Warn("Failed to get pending polls for dashboard")
	} else {
		dashboard.PendingPolls = u.convertPollsToResponses(pendingPolls)
		dashboard.Counts.PendingPollsCount = pendingPollsCount
	}

	// Get today's events from calendar service
	todayEvents, todayEventsCount, err := u.calendarClient.GetTodayEventsForUser(userID, limit)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Warn("Failed to get today's events for dashboard")
	} else {
		dashboard.TodayEvents = u.convertEventsToResponses(todayEvents)
		dashboard.Counts.TodayEventsCount = todayEventsCount
	}

	return dashboard, nil
}

// convertTasksToResponses converts Task models to TaskResponse
func (u *dashboardUsecase) convertTasksToResponses(tasks []*models.Task, userID uint) []*models.TaskResponse {
	if len(tasks) == 0 {
		return []*models.TaskResponse{}
	}

	// Collect all user IDs for batch fetch
	userIDs := make(map[uint]bool)
	for _, task := range tasks {
		userIDs[task.CreatedByUserID] = true
		if task.AssignedToUserID != nil {
			userIDs[*task.AssignedToUserID] = true
		}
		for _, assignee := range task.Assignees {
			userIDs[assignee.UserID] = true
		}
	}

	// Convert map to slice
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	// Fetch user info
	userInfoMap, err := u.userClient.GetUsersByIDs(ids)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Warn("Failed to fetch user info for dashboard tasks")
		userInfoMap = make(map[uint]*clients.UserInfo)
	}

	// Convert to responses
	responses := make([]*models.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		response := task.ToResponse()

		// Enrich with user info
		if userInfo, ok := userInfoMap[task.CreatedByUserID]; ok {
			response.Creator = &models.UserInfo{
				ID:       userInfo.ID,
				Name:     userInfo.Name,
				Email:    userInfo.Email,
				Avatar:   userInfo.Avatar,
				Position: userInfo.Position,
			}
		}

		if task.AssignedToUserID != nil {
			if userInfo, ok := userInfoMap[*task.AssignedToUserID]; ok {
				response.AssignedToUser = &models.UserInfo{
					ID:       userInfo.ID,
					Name:     userInfo.Name,
					Email:    userInfo.Email,
					Avatar:   userInfo.Avatar,
					Position: userInfo.Position,
				}
			}
		}

		// Enrich assignees
		assignees := make([]models.UserInfo, 0, len(task.Assignees))
		for _, assignee := range task.Assignees {
			if userInfo, ok := userInfoMap[assignee.UserID]; ok {
				assignees = append(assignees, models.UserInfo{
					ID:       userInfo.ID,
					Name:     userInfo.Name,
					Email:    userInfo.Email,
					Avatar:   userInfo.Avatar,
					Position: userInfo.Position,
				})
			}
		}
		response.Assignees = assignees

		responses = append(responses, response)
	}

	return responses
}

// convertPollsToResponses converts PendingPollResponse from client to model
func (u *dashboardUsecase) convertPollsToResponses(polls []*clients.PendingPollResponse) []*models.PendingPollResponse {
	if len(polls) == 0 {
		return []*models.PendingPollResponse{}
	}

	responses := make([]*models.PendingPollResponse, 0, len(polls))
	for _, poll := range polls {
		endTime := ""
		if poll.EndTime != nil {
			endTime = *poll.EndTime
		}
		responses = append(responses, &models.PendingPollResponse{
			ID:          poll.ID,
			Title:       poll.Title,
			Description: poll.Description,
			Type:        poll.Type,
			Status:      poll.Status,
			CreatedBy:   poll.CreatedBy,
			CreatedAt:   poll.CreatedAt,
			EndTime:     endTime,
		})
	}

	return responses
}

// convertEventsToResponses converts TodayEventResponse from client to model
func (u *dashboardUsecase) convertEventsToResponses(events []*clients.TodayEventResponse) []*models.TodayEventResponse {
	if len(events) == 0 {
		return []*models.TodayEventResponse{}
	}

	responses := make([]*models.TodayEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, &models.TodayEventResponse{
			ID:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime,
			EndTime:     event.EndTime,
			AllDay:      event.AllDay,
			Location:    event.Location,
			Type:        event.Type,
			Color:       event.Color,
			IsPrivate:   event.IsPrivate,
		})
	}

	return responses
}
