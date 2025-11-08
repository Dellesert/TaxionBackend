package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
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

	// Check if department head verification is required
	if requiresCheck, exists := c.Get("requires_department_head_check"); exists && requiresCheck.(bool) {
		userID, _ := c.Get("user_id")

		// Get department to verify user is the head
		dept, err := h.departmentUsecase.GetDepartment(uint(id))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "Department not found",
				"request_id": requestID,
			})
			return
		}

		// Check if user is the department head
		if dept.HeadID == nil || *dept.HeadID != userID.(uint) {
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"user_id":       userID,
				"department_id": id,
				"head_id":       dept.HeadID,
			}).Warn("User is not the head of this department")

			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Forbidden",
				"message":    "You are not the head of this department",
				"request_id": requestID,
			})
			return
		}
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

// ImportDepartments handles CSV import of departments
func (h *DepartmentHandler) ImportDepartments(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get CSV file from form data
	file, err := c.FormFile("file")
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("No file provided for department import")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "CSV file is required",
			"request_id": requestID,
		})
		return
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to open uploaded file")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to read CSV file",
			"request_id": requestID,
		})
		return
	}
	defer src.Close()

	// Create CSV reader
	reader := csv.NewReader(src)

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to read CSV header")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid CSV format: unable to read header",
			"request_id": requestID,
		})
		return
	}

	// Remove BOM if present
	if len(headers) > 0 && len(headers[0]) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	// Validate required columns
	requiredColumns := []string{"name"}
	headerMap := make(map[string]int)
	for i, h := range headers {
		cleaned := strings.TrimSpace(strings.ToLower(h))
		headerMap[cleaned] = i
	}

	// Check for required columns and provide helpful error message
	missingColumns := []string{}
	for _, col := range requiredColumns {
		if _, exists := headerMap[col]; !exists {
			missingColumns = append(missingColumns, col)
		}
	}

	if len(missingColumns) > 0 {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"missing_columns":  missingColumns,
			"received_headers": headers,
		}).Warn("CSV missing required columns")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":            fmt.Sprintf("Required columns not found: %v", missingColumns),
			"received_headers": headers,
			"expected_headers": []string{"name"},
			"hint":             "CSV file must have a 'name' column in the first row",
			"request_id":       requestID,
		})
		return
	}

	// Track results
	type ImportError struct {
		Row     int    `json:"row"`
		Name    string `json:"name"`
		Message string `json:"message"`
	}

	var successDepartments []map[string]string
	var errors []ImportError
	rowNum := 1 // Start from 1 (header is row 0)

	// Read and process each row
	for {
		rowNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, ImportError{
				Row:     rowNum,
				Name:    "",
				Message: fmt.Sprintf("Failed to parse row: %v", err),
			})
			continue
		}

		// Extract data from row
		name := ""
		if idx, ok := headerMap["name"]; ok && idx < len(row) {
			name = strings.TrimSpace(row[idx])
		}

		// Note: description and head_id fields are not used in department creation
		// Department model doesn't have description field
		// head_id should be set after creating users via department update endpoint

		// Validate required fields
		if name == "" {
			errors = append(errors, ImportError{
				Row:     rowNum,
				Name:    "",
				Message: "Name is required",
			})
			continue
		}

		// Create department request
		req := &models.CreateDepartmentRequest{
			Name: name,
		}

		// Create department
		_, err = h.departmentUsecase.CreateDepartment(req)
		if err != nil {
			errors = append(errors, ImportError{
				Row:     rowNum,
				Name:    name,
				Message: err.Error(),
			})
			continue
		}

		successDepartments = append(successDepartments, map[string]string{"name": name})
	}

	totalRows := rowNum - 1 // Exclude header
	successCount := len(successDepartments)
	errorCount := len(errors)

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"total_rows":    totalRows,
		"success_count": successCount,
		"error_count":   errorCount,
	}).Info("Department import completed")

	c.JSON(http.StatusOK, gin.H{
		"total_rows":          totalRows,
		"success_count":       successCount,
		"error_count":         errorCount,
		"success_departments": successDepartments,
		"errors":              errors,
		"request_id":          requestID,
	})
}
