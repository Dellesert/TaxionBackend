package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// DepartmentHandler handles HTTP requests for department operations
type DepartmentHandler struct {
	departmentUsecase usecase.DepartmentUsecase
}

// NewDepartmentHandler creates a new department handler
func NewDepartmentHandler(departmentUsecase usecase.DepartmentUsecase) *DepartmentHandler {
	return &DepartmentHandler{
		departmentUsecase: departmentUsecase,
	}
}

// GetDepartments handles getting all departments
func (h *DepartmentHandler) GetDepartments(c *gin.Context) {
	requestID := requestid.Get(c)

	departments, err := h.departmentUsecase.GetAllDepartments()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get departments")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get departments",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":        requestID,
		"departments_count": len(departments),
	}).Info("Departments retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"departments": departments,
		"count":       len(departments),
		"request_id":  requestID,
	})
}

// GetDepartment handles getting a department by ID
func (h *DepartmentHandler) GetDepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": idStr,
			"error":         err.Error(),
		}).Warn("Invalid department ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid department ID",
			"request_id": requestID,
		})
		return
	}

	department, err := h.departmentUsecase.GetDepartment(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": id,
			"error":         err.Error(),
		}).Error("Failed to get department")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get department"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Department not found"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"department_id": id,
		"name":          department.Name,
	}).Info("Department retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"department": department,
		"request_id": requestID,
	})
}

// CreateDepartment handles department creation
func (h *DepartmentHandler) CreateDepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create department")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	department, err := h.departmentUsecase.CreateDepartment(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"name":       req.Name,
			"error":      err.Error(),
		}).Error("Failed to create department")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to create department"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "validation failed") {
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
		"request_id":    requestID,
		"department_id": department.ID,
		"name":          department.Name,
	}).Info("Department created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Department created successfully",
		"department": department,
		"request_id": requestID,
	})
}

// UpdateDepartment handles department update
func (h *DepartmentHandler) UpdateDepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": idStr,
			"error":         err.Error(),
		}).Warn("Invalid department ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid department ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": id,
			"error":         err.Error(),
		}).Warn("Invalid request body for update department")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	department, err := h.departmentUsecase.UpdateDepartment(uint(id), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": id,
			"error":         err.Error(),
		}).Error("Failed to update department")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update department"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Department not found"
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "validation failed") {
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
		"request_id":    requestID,
		"department_id": id,
		"name":          department.Name,
	}).Info("Department updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Department updated successfully",
		"department": department,
		"request_id": requestID,
	})
}

// DeleteDepartment handles department deletion
func (h *DepartmentHandler) DeleteDepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": idStr,
			"error":         err.Error(),
		}).Warn("Invalid department ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid department ID",
			"request_id": requestID,
		})
		return
	}

	err = h.departmentUsecase.DeleteDepartment(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": id,
			"error":         err.Error(),
		}).Error("Failed to delete department")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to delete department"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Department not found"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"department_id": id,
	}).Info("Department deleted successfully")

	c.JSON(http.StatusNoContent, gin.H{
		"message":    "Department deleted successfully",
		"request_id": requestID,
	})
}

// GetDepartmentWithUsers handles getting a department with its users
func (h *DepartmentHandler) GetDepartmentWithUsers(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": idStr,
			"error":         err.Error(),
		}).Warn("Invalid department ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid department ID",
			"request_id": requestID,
		})
		return
	}

	department, err := h.departmentUsecase.GetDepartmentWithUsers(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": id,
			"error":         err.Error(),
		}).Error("Failed to get department with users")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get department with users"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Department not found"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"department_id": id,
		"name":          department.Name,
		"user_count":    department.UserCount,
	}).Info("Department with users retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"department": department,
		"request_id": requestID,
	})
}
