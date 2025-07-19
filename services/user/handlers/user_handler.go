package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// UserHandler handles HTTP requests for user operations
type UserHandler struct {
	userUsecase usecase.UserUsecase
}

// NewUserHandler creates a new user handler
func NewUserHandler(userUsecase usecase.UserUsecase) *UserHandler {
	return &UserHandler{
		userUsecase: userUsecase,
	}
}

// CreateUser handles user creation requests
func (h *UserHandler) CreateUser(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.CreateUser(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Error("Failed to create user")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to create user",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    user.ID,
		"email":      user.Email,
	}).Info("User created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"user":       user,
		"request_id": requestID,
	})
}

// GetUser handles getting a single user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.GetUser(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to get user")

		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":       user,
		"request_id": requestID,
	})
}

// GetUsers handles getting all users with pagination
func (h *UserHandler) GetUsers(c *gin.Context) {
	requestID := requestid.Get(c)

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	users, total, err := h.userUsecase.GetUsers(limit, offset)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"limit":      limit,
			"offset":     offset,
			"error":      err.Error(),
		}).Error("Failed to get users")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get users",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":      users,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"request_id": requestID,
	})
}

// UpdateUser handles user update requests
func (h *UserHandler) UpdateUser(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Warn("Invalid request body for update user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.UpdateUser(uint(id), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to update user")

		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    id,
	}).Info("User updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"user":       user,
		"request_id": requestID,
	})
}

// DeleteUser handles user deletion requests
func (h *UserHandler) DeleteUser(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	if err := h.userUsecase.DeleteUser(uint(id)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to delete user")

		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    id,
	}).Info("User deleted successfully")

	c.JSON(http.StatusNoContent, gin.H{
		"request_id": requestID,
	})
}
