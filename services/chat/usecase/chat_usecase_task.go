package usecase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/services/chat/models"
)

// TaskInfo represents task information from task-service
type TaskInfo struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	CreatorID   uint   `json:"created_by"`
	Assignees   []TaskAssignee `json:"assignees"`
}

// TaskAssignee represents an assignee of a task
type TaskAssignee struct {
	UserID uint `json:"user_id"`
}

// GetOrCreateDirectChat gets existing or creates new direct chat with a user
func (uc *chatUsecase) GetOrCreateDirectChat(userID, targetUserID uint) (*models.ChatResponse, error) {
	// Validate input
	if userID == targetUserID {
		return nil, fmt.Errorf("cannot create personal chat with yourself")
	}

	// Check if personal chat already exists between these users
	existingChat, err := uc.chatRepo.GetDirectChatBetweenUsers(userID, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing direct chat: %w", err)
	}

	// If chat exists, return it
	if existingChat != nil {
		// Get unread count for this chat
		response := existingChat.ToResponse(uc.baseURL)
		unreadCount, err := uc.messageRepo.GetUnreadCount(existingChat.ID, userID)
		if err == nil {
			response.UnreadCount = unreadCount
		}

		// Get favorite and pinned status for current user
		response.IsFavorite = false
		response.IsPinned = false
		for _, member := range existingChat.Members {
			if member.UserID == userID {
				response.IsFavorite = member.IsFavorite
				response.IsPinned = member.IsPinned
				break
			}
		}

		return response, nil
	}

	// Create new personal chat using existing method
	return uc.CreatePersonalChat(userID, targetUserID)
}

// GetOrCreateTaskChat gets existing or creates new group chat for a task
func (uc *chatUsecase) GetOrCreateTaskChat(userID, taskID uint) (*models.ChatResponse, error) {
	// Check if task chat already exists
	// GetChatByTaskID returns (nil, nil) if chat not found, (nil, err) if real error
	existingChat, err := uc.chatRepo.GetChatByTaskID(taskID)
	if err != nil {
		// Real error occurred (not just "not found")
		return nil, fmt.Errorf("failed to check existing task chat: %w", err)
	}

	// If chat exists (not nil), check if user is member, if not - add them
	if existingChat != nil {
		isMember, err := uc.chatRepo.IsMember(existingChat.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to check membership: %w", err)
		}

		// If user is not member, add them
		if !isMember {
			member := &models.ChatMember{
				ChatID:   existingChat.ID,
				UserID:   userID,
				Role:     models.ChatMemberRoleMember,
				JoinedAt: time.Now(),
				IsActive: true,
			}
			if err := uc.chatRepo.AddMember(member); err != nil {
				return nil, fmt.Errorf("failed to add user to task chat: %w", err)
			}

			// Reload chat with updated members
			existingChat, err = uc.chatRepo.GetWithMembers(existingChat.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to reload task chat: %w", err)
			}
		}

		// Get unread count and return
		response := existingChat.ToResponse(uc.baseURL)
		unreadCount, err := uc.messageRepo.GetUnreadCount(existingChat.ID, userID)
		if err == nil {
			response.UnreadCount = unreadCount
		}

		// Get favorite and pinned status for current user
		response.IsFavorite = false
		response.IsPinned = false
		for _, member := range existingChat.Members {
			if member.UserID == userID {
				response.IsFavorite = member.IsFavorite
				response.IsPinned = member.IsPinned
				break
			}
		}

		return response, nil
	}

	// Chat doesn't exist, fetch task info and create new chat
	taskInfo, err := uc.fetchTaskInfo(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task info: %w", err)
	}

	// Create task chat name
	chatName := fmt.Sprintf("Обсуждение: %s", taskInfo.Title)
	if len(chatName) > 255 {
		chatName = chatName[:252] + "..."
	}

	// Collect unique member IDs (creator + assignees + current user)
	memberIDMap := make(map[uint]bool)
	memberIDMap[taskInfo.CreatorID] = true
	memberIDMap[userID] = true
	for _, assignee := range taskInfo.Assignees {
		memberIDMap[assignee.UserID] = true
	}

	memberIDs := make([]uint, 0, len(memberIDMap))
	for id := range memberIDMap {
		memberIDs = append(memberIDs, id)
	}

	// Create group chat
	chat := &models.Chat{
		Name:        chatName,
		Description: fmt.Sprintf("Обсуждение задачи #%d", taskID),
		Type:        models.ChatTypeGroup,
		CreatorID:   userID,
		IsActive:    true,
		TaskID:      &taskID,
	}

	if err := uc.chatRepo.Create(chat); err != nil {
		return nil, fmt.Errorf("failed to create task chat: %w", err)
	}

	// Add all members to the chat
	for _, memberID := range memberIDs {
		// Skip creator - already added by AfterCreate hook
		if memberID == userID {
			continue
		}

		member := &models.ChatMember{
			ChatID:   chat.ID,
			UserID:   memberID,
			Role:     models.ChatMemberRoleMember,
			JoinedAt: time.Now(),
			IsActive: true,
		}

		if err := uc.chatRepo.AddMember(member); err != nil {
			// Log error but continue adding other members
			fmt.Printf("Warning: failed to add member %d to task chat: %v\n", memberID, err)
			continue
		}
	}

	// Create context message (optional)
	contextMessage := &models.Message{
		ChatID:    chat.ID,
		SenderID:  userID,
		Content:   fmt.Sprintf("📋 Обсуждение задачи: %s\n\nИспользуйте этот чат для обсуждения задачи.", taskInfo.Title),
		Type:      models.MessageTypeText,
		IsDeleted: false,
	}
	if err := uc.messageRepo.Create(contextMessage); err != nil {
		// Non-critical error, just log it
		fmt.Printf("Warning: failed to create context message for task chat: %v\n", err)
	}

	// Get chat with members for response
	chatWithMembers, err := uc.chatRepo.GetWithMembers(chat.ID)
	if err != nil {
		return chat.ToResponse(uc.baseURL), nil // Return what we have
	}

	return chatWithMembers.ToResponse(uc.baseURL), nil
}

// fetchTaskInfo fetches task information from task-service
func (uc *chatUsecase) fetchTaskInfo(taskID uint) (*TaskInfo, error) {
	taskServiceURL := os.Getenv("TASK_SERVICE_URL")
	if taskServiceURL == "" {
		taskServiceURL = "http://task-service:8084"
	}

	// Make request to task-service
	url := fmt.Sprintf("%s/api/v1/internal/tasks/%d", taskServiceURL, taskID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("task service returned status %d: %s", resp.StatusCode, string(body))
	}

	var taskInfo TaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&taskInfo); err != nil {
		return nil, fmt.Errorf("failed to decode task info: %w", err)
	}

	return &taskInfo, nil
}
