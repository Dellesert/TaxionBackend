package handlers

import (
	"fmt"
	"net/http"
	"time"

	"tachyon-messenger/services/search/models"
	"tachyon-messenger/services/search/usecase"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// ReindexHandler handles full reindexing of all entities
type ReindexHandler struct {
	db            *database.DB
	searchUsecase usecase.SearchUsecase
}

// NewReindexHandler creates a new reindex handler
func NewReindexHandler(db *database.DB, searchUsecase usecase.SearchUsecase) *ReindexHandler {
	return &ReindexHandler{db: db, searchUsecase: searchUsecase}
}

// ReindexAll handles POST /api/v1/internal/search/reindex
func (h *ReindexHandler) ReindexAll(c *gin.Context) {
	requestID := requestid.Get(c)
	start := time.Now()

	logger.Info("[Reindex] Starting full reindex of all entities...")

	stats := map[string]int{
		"task":     0,
		"chat":     0,
		"message":  0,
		"poll":     0,
		"event":    0,
		"schedule": 0,
	}
	errors := []string{}

	// 1. Index tasks
	taskCount, err := h.reindexTasks()
	if err != nil {
		errors = append(errors, fmt.Sprintf("tasks: %v", err))
		logger.Errorf("[Reindex] Failed to reindex tasks: %v", err)
	} else {
		stats["task"] = taskCount
		logger.Infof("[Reindex] Indexed %d tasks", taskCount)
	}

	// 2. Index chats
	chatCount, err := h.reindexChats()
	if err != nil {
		errors = append(errors, fmt.Sprintf("chats: %v", err))
		logger.Errorf("[Reindex] Failed to reindex chats: %v", err)
	} else {
		stats["chat"] = chatCount
		logger.Infof("[Reindex] Indexed %d chats", chatCount)
	}

	// 3. Index messages
	messageCount, err := h.reindexMessages()
	if err != nil {
		errors = append(errors, fmt.Sprintf("messages: %v", err))
		logger.Errorf("[Reindex] Failed to reindex messages: %v", err)
	} else {
		stats["message"] = messageCount
		logger.Infof("[Reindex] Indexed %d messages", messageCount)
	}

	// 4. Index polls
	pollCount, err := h.reindexPolls()
	if err != nil {
		errors = append(errors, fmt.Sprintf("polls: %v", err))
		logger.Errorf("[Reindex] Failed to reindex polls: %v", err)
	} else {
		stats["poll"] = pollCount
		logger.Infof("[Reindex] Indexed %d polls", pollCount)
	}

	// 5. Index events
	eventCount, err := h.reindexEvents()
	if err != nil {
		errors = append(errors, fmt.Sprintf("events: %v", err))
		logger.Errorf("[Reindex] Failed to reindex events: %v", err)
	} else {
		stats["event"] = eventCount
		logger.Infof("[Reindex] Indexed %d events", eventCount)
	}

	// 6. Index schedules
	scheduleCount, err := h.reindexSchedules()
	if err != nil {
		errors = append(errors, fmt.Sprintf("schedules: %v", err))
		logger.Errorf("[Reindex] Failed to reindex schedules: %v", err)
	} else {
		stats["schedule"] = scheduleCount
		logger.Infof("[Reindex] Indexed %d schedules", scheduleCount)
	}

	duration := time.Since(start)
	total := 0
	for _, v := range stats {
		total += v
	}

	logger.WithFields(map[string]interface{}{
		"total":    total,
		"stats":    stats,
		"duration": duration.String(),
		"errors":   len(errors),
	}).Info("[Reindex] Full reindex completed")

	c.JSON(http.StatusOK, gin.H{
		"status":     "completed",
		"request_id": requestID,
		"total":      total,
		"stats":      stats,
		"errors":     errors,
		"duration":   duration.String(),
	})
}

// fetchUserAvatars fetches avatar info for a list of user IDs from the users table
func (h *ReindexHandler) fetchUserAvatars(userIDs []uint) map[uint]struct{ Name, Avatar string } {
	result := make(map[uint]struct{ Name, Avatar string })
	if len(userIDs) == 0 {
		return result
	}

	type userRow struct {
		ID     uint
		Name   string
		Avatar string
	}
	var users []userRow
	h.db.Raw("SELECT id, COALESCE(name, '') as name, COALESCE(avatar, '') as avatar FROM users WHERE id IN ? AND deleted_at IS NULL", userIDs).Scan(&users)

	for _, u := range users {
		result[u.ID] = struct{ Name, Avatar string }{Name: u.Name, Avatar: u.Avatar}
	}
	return result
}

// reindexTasks reads all tasks and indexes them
func (h *ReindexHandler) reindexTasks() (int, error) {
	type taskRow struct {
		ID                   uint
		Title                string
		Description          string
		Status               string
		Priority             string
		CreatedByUserID      uint
		AssignedToDepartment *uint
		DueDate              *time.Time
	}

	var tasks []taskRow
	if err := h.db.Raw(`
		SELECT id, title, description, status, priority, created_by_user_id, assigned_to_department_id, due_date
		FROM tasks
		WHERE deleted_at IS NULL
	`).Scan(&tasks).Error; err != nil {
		return 0, fmt.Errorf("failed to query tasks: %w", err)
	}

	// Batch-fetch creator avatars
	creatorIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		creatorIDs = append(creatorIDs, t.CreatedByUserID)
	}
	avatarCache := h.fetchUserAvatars(creatorIDs)

	count := 0
	for _, task := range tasks {
		// Get assignee IDs
		var assigneeIDs []uint
		h.db.Raw("SELECT user_id FROM task_assignees WHERE task_id = ? AND deleted_at IS NULL", task.ID).Scan(&assigneeIDs)

		accessibleBy := []uint{task.CreatedByUserID}
		for _, id := range assigneeIDs {
			if id != task.CreatedByUserID {
				accessibleBy = append(accessibleBy, id)
			}
		}

		metadata := map[string]interface{}{
			"status":   task.Status,
			"priority": task.Priority,
		}
		if task.DueDate != nil {
			metadata["due_date"] = task.DueDate.Format(time.RFC3339)
		}
		if task.AssignedToDepartment != nil {
			metadata["department_id"] = *task.AssignedToDepartment
		}
		if creator, ok := avatarCache[task.CreatedByUserID]; ok {
			metadata["creator_name"] = creator.Name
			metadata["creator_avatar"] = creator.Avatar
		}

		if err := h.searchUsecase.IndexDocument(&models.IndexDocumentRequest{
			EntityType:   models.EntityTypeTask,
			EntityID:     task.ID,
			Title:        task.Title,
			Content:      task.Description,
			Metadata:     metadata,
			AccessibleBy: accessibleBy,
			IsPublic:     false,
			CreatorID:    task.CreatedByUserID,
		}); err != nil {
			logger.Errorf("[Reindex] Failed to index task %d: %v", task.ID, err)
			continue
		}
		count++
	}

	return count, nil
}

// reindexChats reads all chats and indexes them
func (h *ReindexHandler) reindexChats() (int, error) {
	type chatRow struct {
		ID          uint
		Name        string
		Description string
		Type        string
		CreatorID   uint
		Avatar      string
	}

	var chats []chatRow
	if err := h.db.Raw(`
		SELECT id, name, description, type, creator_id, COALESCE(avatar, '') as avatar
		FROM chats
		WHERE deleted_at IS NULL AND is_active = true
	`).Scan(&chats).Error; err != nil {
		return 0, fmt.Errorf("failed to query chats: %w", err)
	}

	count := 0
	for _, chat := range chats {
		// Get active member IDs
		var memberIDs []uint
		h.db.Raw("SELECT user_id FROM chat_members WHERE chat_id = ? AND is_active = true AND deleted_at IS NULL", chat.ID).Scan(&memberIDs)

		metadata := map[string]interface{}{
			"type":   chat.Type,
			"avatar": chat.Avatar,
		}

		if err := h.searchUsecase.IndexDocument(&models.IndexDocumentRequest{
			EntityType:   models.EntityTypeChat,
			EntityID:     chat.ID,
			Title:        chat.Name,
			Content:      chat.Description,
			Metadata:     metadata,
			AccessibleBy: memberIDs,
			IsPublic:     false,
			CreatorID:    chat.CreatorID,
		}); err != nil {
			logger.Errorf("[Reindex] Failed to index chat %d: %v", chat.ID, err)
			continue
		}
		count++
	}

	return count, nil
}

// reindexMessages reads all messages and indexes them
func (h *ReindexHandler) reindexMessages() (int, error) {
	type messageRow struct {
		ID       uint
		ChatID   uint
		SenderID uint
		Content  string
		Type     string
		FileName string
	}

	var messages []messageRow
	if err := h.db.Raw(`
		SELECT id, chat_id, sender_id, content, type, COALESCE(file_name, '') as file_name
		FROM messages
		WHERE deleted_at IS NULL AND is_deleted = false AND content != ''
	`).Scan(&messages).Error; err != nil {
		return 0, fmt.Errorf("failed to query messages: %w", err)
	}

	// Batch-fetch sender avatars
	senderIDs := make([]uint, 0, len(messages))
	for _, m := range messages {
		senderIDs = append(senderIDs, m.SenderID)
	}
	avatarCache := h.fetchUserAvatars(senderIDs)

	// Pre-fetch chat member IDs for all relevant chats
	chatMembersCache := map[uint][]uint{}

	count := 0
	for _, msg := range messages {
		memberIDs, ok := chatMembersCache[msg.ChatID]
		if !ok {
			h.db.Raw("SELECT user_id FROM chat_members WHERE chat_id = ? AND is_active = true AND deleted_at IS NULL", msg.ChatID).Scan(&memberIDs)
			chatMembersCache[msg.ChatID] = memberIDs
		}

		content := msg.Content
		if msg.FileName != "" {
			content = content + " " + msg.FileName
		}

		metadata := map[string]interface{}{
			"chat_id":   msg.ChatID,
			"sender_id": msg.SenderID,
			"type":      msg.Type,
		}
		if sender, ok := avatarCache[msg.SenderID]; ok {
			metadata["sender_name"] = sender.Name
			metadata["sender_avatar"] = sender.Avatar
		}

		if err := h.searchUsecase.IndexDocument(&models.IndexDocumentRequest{
			EntityType:   models.EntityTypeMessage,
			EntityID:     msg.ID,
			Title:        "",
			Content:      content,
			Metadata:     metadata,
			AccessibleBy: memberIDs,
			IsPublic:     false,
			CreatorID:    msg.SenderID,
		}); err != nil {
			logger.Errorf("[Reindex] Failed to index message %d: %v", msg.ID, err)
			continue
		}
		count++
	}

	return count, nil
}

// reindexPolls reads all polls and indexes them
func (h *ReindexHandler) reindexPolls() (int, error) {
	type pollRow struct {
		ID           uint
		Title        string
		Description  string
		Type         string
		Status       string
		Visibility   string
		CreatedBy    uint
		DepartmentID *uint
		Category     string
	}

	var polls []pollRow
	if err := h.db.Raw(`
		SELECT id, title, description, type, status, visibility, created_by,
		       department_id, COALESCE(category, '') as category
		FROM polls
		WHERE deleted_at IS NULL
	`).Scan(&polls).Error; err != nil {
		return 0, fmt.Errorf("failed to query polls: %w", err)
	}

	// Batch-fetch creator avatars
	pollCreatorIDs := make([]uint, 0, len(polls))
	for _, p := range polls {
		pollCreatorIDs = append(pollCreatorIDs, p.CreatedBy)
	}
	pollAvatarCache := h.fetchUserAvatars(pollCreatorIDs)

	count := 0
	for _, poll := range polls {
		// Get participant IDs
		var participantIDs []uint
		h.db.Raw("SELECT user_id FROM poll_participants WHERE poll_id = ? AND deleted_at IS NULL", poll.ID).Scan(&participantIDs)

		accessibleBy := []uint{poll.CreatedBy}
		for _, id := range participantIDs {
			if id != poll.CreatedBy {
				accessibleBy = append(accessibleBy, id)
			}
		}

		metadata := map[string]interface{}{
			"status":     poll.Status,
			"type":       poll.Type,
			"visibility": poll.Visibility,
		}
		if poll.DepartmentID != nil {
			metadata["department_id"] = *poll.DepartmentID
		}
		if poll.Category != "" {
			metadata["category"] = poll.Category
		}
		if creator, ok := pollAvatarCache[poll.CreatedBy]; ok {
			metadata["creator_name"] = creator.Name
			metadata["creator_avatar"] = creator.Avatar
		}

		isPublic := poll.Visibility == "public" || poll.Visibility == "department"

		if err := h.searchUsecase.IndexDocument(&models.IndexDocumentRequest{
			EntityType:   models.EntityTypePoll,
			EntityID:     poll.ID,
			Title:        poll.Title,
			Content:      poll.Description,
			Metadata:     metadata,
			AccessibleBy: accessibleBy,
			IsPublic:     isPublic,
			CreatorID:    poll.CreatedBy,
		}); err != nil {
			logger.Errorf("[Reindex] Failed to index poll %d: %v", poll.ID, err)
			continue
		}
		count++
	}

	return count, nil
}

// reindexEvents reads all events and indexes them
func (h *ReindexHandler) reindexEvents() (int, error) {
	type eventRow struct {
		ID          uint
		Title       string
		Description string
		Location    string
		Type        string
		CreatedBy   uint
		IsPrivate   bool
		AllDay      bool
	}

	var events []eventRow
	if err := h.db.Raw(`
		SELECT id, title, description, COALESCE(location, '') as location,
		       type, created_by, is_private, all_day
		FROM events
		WHERE deleted_at IS NULL
	`).Scan(&events).Error; err != nil {
		return 0, fmt.Errorf("failed to query events: %w", err)
	}

	// Batch-fetch creator avatars
	eventCreatorIDs := make([]uint, 0, len(events))
	for _, e := range events {
		eventCreatorIDs = append(eventCreatorIDs, e.CreatedBy)
	}
	eventAvatarCache := h.fetchUserAvatars(eventCreatorIDs)

	count := 0
	for _, event := range events {
		// Get participant IDs
		var participantIDs []uint
		h.db.Raw("SELECT user_id FROM event_participants WHERE event_id = ? AND deleted_at IS NULL", event.ID).Scan(&participantIDs)

		accessibleBy := []uint{event.CreatedBy}
		for _, id := range participantIDs {
			if id != event.CreatedBy {
				accessibleBy = append(accessibleBy, id)
			}
		}

		metadata := map[string]interface{}{
			"type":     event.Type,
			"location": event.Location,
		}
		if event.AllDay {
			metadata["all_day"] = true
		}
		if creator, ok := eventAvatarCache[event.CreatedBy]; ok {
			metadata["creator_name"] = creator.Name
			metadata["creator_avatar"] = creator.Avatar
		}

		content := event.Description
		if event.Location != "" {
			content += " " + event.Location
		}

		if err := h.searchUsecase.IndexDocument(&models.IndexDocumentRequest{
			EntityType:   models.EntityTypeEvent,
			EntityID:     event.ID,
			Title:        event.Title,
			Content:      content,
			Metadata:     metadata,
			AccessibleBy: accessibleBy,
			IsPublic:     !event.IsPrivate,
			CreatorID:    event.CreatedBy,
		}); err != nil {
			logger.Errorf("[Reindex] Failed to index event %d: %v", event.ID, err)
			continue
		}
		count++
	}

	return count, nil
}

// reindexSchedules reads all schedules and indexes them
func (h *ReindexHandler) reindexSchedules() (int, error) {
	type scheduleRow struct {
		ID            uint
		Title         string
		Description   string
		Type          string
		Visibility    string
		CreatedBy     uint
		IsForAllUsers bool
		IsActive      bool
		DepartmentID  *uint
	}

	var schedules []scheduleRow
	if err := h.db.Raw(`
		SELECT id, title, description, type, visibility, created_by,
		       is_for_all_users, is_active, department_id
		FROM schedules
		WHERE deleted_at IS NULL
	`).Scan(&schedules).Error; err != nil {
		return 0, fmt.Errorf("failed to query schedules: %w", err)
	}

	// Batch-fetch creator avatars
	scheduleCreatorIDs := make([]uint, 0, len(schedules))
	for _, s := range schedules {
		scheduleCreatorIDs = append(scheduleCreatorIDs, s.CreatedBy)
	}
	scheduleAvatarCache := h.fetchUserAvatars(scheduleCreatorIDs)

	count := 0
	for _, schedule := range schedules {
		seen := map[uint]bool{schedule.CreatedBy: true}
		accessibleBy := []uint{schedule.CreatedBy}

		// Viewers
		var viewerIDs []uint
		h.db.Raw("SELECT user_id FROM schedule_viewers WHERE schedule_id = ? AND deleted_at IS NULL", schedule.ID).Scan(&viewerIDs)
		for _, id := range viewerIDs {
			if !seen[id] {
				accessibleBy = append(accessibleBy, id)
				seen[id] = true
			}
		}

		// Assignments
		var assignmentIDs []uint
		h.db.Raw("SELECT user_id FROM schedule_assignments WHERE schedule_id = ? AND deleted_at IS NULL", schedule.ID).Scan(&assignmentIDs)
		for _, id := range assignmentIDs {
			if !seen[id] {
				accessibleBy = append(accessibleBy, id)
				seen[id] = true
			}
		}

		// Editors
		var editorIDs []uint
		h.db.Raw("SELECT user_id FROM schedule_editors WHERE schedule_id = ? AND deleted_at IS NULL", schedule.ID).Scan(&editorIDs)
		for _, id := range editorIDs {
			if !seen[id] {
				accessibleBy = append(accessibleBy, id)
				seen[id] = true
			}
		}

		metadata := map[string]interface{}{
			"type":       schedule.Type,
			"visibility": schedule.Visibility,
			"is_active":  schedule.IsActive,
		}
		if schedule.DepartmentID != nil {
			metadata["department_id"] = *schedule.DepartmentID
		}

		isPublic := schedule.Visibility == "all" || schedule.IsForAllUsers

		if err := h.searchUsecase.IndexDocument(&models.IndexDocumentRequest{
			EntityType:   models.EntityTypeSchedule,
			EntityID:     schedule.ID,
			Title:        schedule.Title,
			Content:      schedule.Description,
			Metadata:     metadata,
			AccessibleBy: accessibleBy,
			IsPublic:     isPublic,
			CreatorID:    schedule.CreatedBy,
		}); err != nil {
			logger.Errorf("[Reindex] Failed to index schedule %d: %v", schedule.ID, err)
			continue
		}
		count++
	}

	return count, nil
}
