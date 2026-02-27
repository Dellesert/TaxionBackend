package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// UserGroupHandler handles HTTP requests for user group operations
type UserGroupHandler struct {
	userGroupUsecase usecase.UserGroupUsecase
}

// NewUserGroupHandler creates a new user group handler
func NewUserGroupHandler(userGroupUsecase usecase.UserGroupUsecase) *UserGroupHandler {
	return &UserGroupHandler{
		userGroupUsecase: userGroupUsecase,
	}
}

// GetGroups handles getting all user groups
func (h *UserGroupHandler) GetGroups(c *gin.Context) {
	requestID := requestid.Get(c)

	// Check if full member data is requested (for UserSelectorModal)
	withMembers := c.Query("with_members") == "true"

	if withMembers {
		groups, err := h.userGroupUsecase.GetAllGroupsWithMembers()
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"error":      err.Error(),
			}).Error("Failed to get user groups with members")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Не удалось получить группы пользователей",
				"request_id": requestID,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"groups":     groups,
			"count":      len(groups),
			"request_id": requestID,
		})
		return
	}

	groups, err := h.userGroupUsecase.GetAllGroups()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user groups")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить группы пользователей",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"groups_count": len(groups),
	}).Info("User groups retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"groups":     groups,
		"count":      len(groups),
		"request_id": requestID,
	})
}

// GetGroup handles getting a user group by ID with its members
func (h *UserGroupHandler) GetGroup(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID группы",
			"request_id": requestID,
		})
		return
	}

	group, err := h.userGroupUsecase.GetGroup(uint(id))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось получить группу пользователей"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Группа пользователей не найдена"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"group":      group,
		"request_id": requestID,
	})
}

// CreateGroup handles user group creation
func (h *UserGroupHandler) CreateGroup(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Требуется аутентификация",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	group, err := h.userGroupUsecase.CreateGroup(&req, userID.(uint))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось создать группу пользователей"

		if strings.Contains(err.Error(), "validation") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must be") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"group_id":   group.ID,
		"name":       group.Name,
		"creator_id": userID,
	}).Info("User group created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "User group created successfully",
		"group":      group,
		"request_id": requestID,
	})
}

// UpdateGroup handles user group update
func (h *UserGroupHandler) UpdateGroup(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID группы",
			"request_id": requestID,
		})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var req models.UpdateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	group, err := h.userGroupUsecase.UpdateGroup(uint(id), &req, userID.(uint), string(userRole.(sharedmodels.Role)))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось обновить группу пользователей"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Группа пользователей не найдена"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "validation") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "cannot be empty") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"group_id":   id,
		"name":       group.Name,
	}).Info("User group updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User group updated successfully",
		"group":      group,
		"request_id": requestID,
	})
}

// DeleteGroup handles user group deletion
func (h *UserGroupHandler) DeleteGroup(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID группы",
			"request_id": requestID,
		})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	err = h.userGroupUsecase.DeleteGroup(uint(id), userID.(uint), string(userRole.(sharedmodels.Role)))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось удалить группу пользователей"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Группа пользователей не найдена"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"group_id":   id,
	}).Info("User group deleted successfully")

	c.JSON(http.StatusNoContent, gin.H{
		"message":    "User group deleted successfully",
		"request_id": requestID,
	})
}

// UpdateMembers handles replacing all members of a user group
func (h *UserGroupHandler) UpdateMembers(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID группы",
			"request_id": requestID,
		})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var req models.UpdateUserGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	group, err := h.userGroupUsecase.SetMembers(uint(id), &req, userID.(uint), string(userRole.(sharedmodels.Role)))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось обновить участников группы"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Группа пользователей не найдена"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"group_id":     id,
		"member_count": group.MemberCount,
	}).Info("User group members updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Group members updated successfully",
		"group":      group,
		"request_id": requestID,
	})
}

// AddMembers handles adding members to a user group
func (h *UserGroupHandler) AddMembers(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID группы",
			"request_id": requestID,
		})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var req models.AddRemoveUserGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.userGroupUsecase.AddMembers(uint(id), &req, userID.(uint), string(userRole.(sharedmodels.Role)))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось добавить участников в группу"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Группа пользователей не найдена"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"group_id":    id,
		"added_count": len(req.UserIDs),
	}).Info("Members added to user group successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Members added successfully",
		"request_id": requestID,
	})
}

// ReorderGroups handles reordering user groups
func (h *UserGroupHandler) ReorderGroups(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.ReorderUserGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err := h.userGroupUsecase.ReorderGroups(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to reorder user groups")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось переупорядочить группы пользователей",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"groups_count": len(req.GroupIDs),
	}).Info("User groups reordered successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User groups reordered successfully",
		"request_id": requestID,
	})
}

// RemoveMembers handles removing members from a user group
func (h *UserGroupHandler) RemoveMembers(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID группы",
			"request_id": requestID,
		})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var req models.AddRemoveUserGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.userGroupUsecase.RemoveMembers(uint(id), &req, userID.(uint), string(userRole.(sharedmodels.Role)))
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось удалить участников из группы"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Группа пользователей не найдена"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"group_id":      id,
		"removed_count": len(req.UserIDs),
	}).Info("Members removed from user group successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Members removed successfully",
		"request_id": requestID,
	})
}
