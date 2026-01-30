package handlers

import (
	"net/http"
	"strconv"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// SubstitutionHandler handles HTTP requests for substitution operations
type SubstitutionHandler struct {
	absenceUsecase usecase.AbsenceUsecase
}

// NewSubstitutionHandler creates a new substitution handler
func NewSubstitutionHandler(absenceUsecase usecase.AbsenceUsecase) *SubstitutionHandler {
	return &SubstitutionHandler{
		absenceUsecase: absenceUsecase,
	}
}

// GetSubstitutions handles get substitutions for an absence
// GET /api/v1/absences/:id/substitutions
func (h *SubstitutionHandler) GetSubstitutions(c *gin.Context) {
	requestID := requestid.Get(c)

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	substitutions, err := h.absenceUsecase.GetSubstitutions(uint(absenceID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"absence_id": absenceID,
			"error":      err.Error(),
		}).Error("Failed to get substitutions")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get substitutions",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"substitutions": substitutions,
		"absence_id":    absenceID,
		"total":         len(substitutions),
		"request_id":    requestID,
	})
}

// CreateSubstitution handles substitution creation requests
// POST /api/v1/absences/:id/substitutions
func (h *SubstitutionHandler) CreateSubstitution(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateSubstitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"absence_id": absenceID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create substitution")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	substitution, err := h.absenceUsecase.CreateSubstitution(userID, uint(absenceID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"user_id":       userID,
			"absence_id":    absenceID,
			"substitute_id": req.SubstituteID,
			"error":         err.Error(),
		}).Error("Failed to create substitution")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" {
			statusCode = http.StatusNotFound
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create substitution",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"absence_id":      absenceID,
		"substitution_id": substitution.ID,
		"substitute_id":   substitution.SubstituteID,
	}).Info("Substitution created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Substitution created successfully",
		"substitution": substitution,
		"request_id":   requestID,
	})
}

// UpdateSubstitution handles substitution update requests
// PUT /api/v1/absences/:id/substitutions/:sub_id
func (h *SubstitutionHandler) UpdateSubstitution(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	subID, err := strconv.ParseUint(c.Param("sub_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid substitution ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateSubstitutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	substitution, err := h.absenceUsecase.UpdateSubstitution(userID, uint(absenceID), uint(subID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"absence_id":      absenceID,
			"substitution_id": subID,
			"error":           err.Error(),
		}).Error("Failed to update substitution")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" || err.Error() == "substitution not found" {
			statusCode = http.StatusNotFound
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update substitution",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"absence_id":      absenceID,
		"substitution_id": subID,
	}).Info("Substitution updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":      "Substitution updated successfully",
		"substitution": substitution,
		"request_id":   requestID,
	})
}

// DeleteSubstitution handles substitution deletion requests
// DELETE /api/v1/absences/:id/substitutions/:sub_id
func (h *SubstitutionHandler) DeleteSubstitution(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	subID, err := strconv.ParseUint(c.Param("sub_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid substitution ID",
			"request_id": requestID,
		})
		return
	}

	if err := h.absenceUsecase.DeleteSubstitution(userID, uint(absenceID), uint(subID)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"absence_id":      absenceID,
			"substitution_id": subID,
			"error":           err.Error(),
		}).Error("Failed to delete substitution")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" || err.Error() == "substitution not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete substitution",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"absence_id":      absenceID,
		"substitution_id": subID,
	}).Info("Substitution deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Substitution deleted successfully",
		"request_id": requestID,
	})
}

// GetUserSubstitutions handles get substitutions where user is a substitute
// GET /api/v1/users/:id/substitutions
func (h *SubstitutionHandler) GetUserSubstitutions(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// Parse date range (default to current month)
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, -1)

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	substitutions, err := h.absenceUsecase.GetUserSubstitutions(uint(userID), startDate, endDate)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user substitutions")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get user substitutions",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"substitutions": substitutions,
		"user_id":       userID,
		"start_date":    startDate.Format("2006-01-02"),
		"end_date":      endDate.Format("2006-01-02"),
		"total":         len(substitutions),
		"request_id":    requestID,
	})
}
