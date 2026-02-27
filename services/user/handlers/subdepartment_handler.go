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

// SubdepartmentHandler handles HTTP requests for subdepartment operations
type SubdepartmentHandler struct {
	subdepartmentUsecase usecase.SubdepartmentUsecase
	departmentUsecase    usecase.DepartmentUsecase
}

// NewSubdepartmentHandler creates a new subdepartment handler
func NewSubdepartmentHandler(subdepartmentUsecase usecase.SubdepartmentUsecase, departmentUsecase usecase.DepartmentUsecase) *SubdepartmentHandler {
	return &SubdepartmentHandler{
		subdepartmentUsecase: subdepartmentUsecase,
		departmentUsecase:    departmentUsecase,
	}
}

// GetSubdepartments handles getting all subdepartments
func (h *SubdepartmentHandler) GetSubdepartments(c *gin.Context) {
	requestID := requestid.Get(c)

	// Check if filtering by department_id
	departmentIDStr := c.Query("department_id")
	if departmentIDStr != "" {
		departmentID, err := strconv.ParseUint(departmentIDStr, 10, 32)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"department_id": departmentIDStr,
				"error":         err.Error(),
			}).Warn("Invalid department ID")

			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Неверный ID отдела",
				"request_id": requestID,
			})
			return
		}

		// Get subdepartments for specific department
		subdepartments, err := h.subdepartmentUsecase.GetSubdepartmentsByDepartment(uint(departmentID))
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"department_id": departmentID,
				"error":         err.Error(),
			}).Error("Failed to get subdepartments by department")

			statusCode := http.StatusInternalServerError
			errorMessage := "Не удалось получить подотделы"

			if strings.Contains(err.Error(), "not found") {
				statusCode = http.StatusNotFound
				errorMessage = "Отдел не найден"
			}

			c.JSON(statusCode, gin.H{
				"error":      errorMessage,
				"request_id": requestID,
			})
			return
		}

		logger.WithFields(map[string]interface{}{
			"request_id":           requestID,
			"department_id":        departmentID,
			"subdepartments_count": len(subdepartments),
		}).Info("Subdepartments retrieved successfully by department")

		c.JSON(http.StatusOK, gin.H{
			"subdepartments": subdepartments,
			"count":          len(subdepartments),
			"request_id":     requestID,
		})
		return
	}

	// Get all subdepartments
	subdepartments, err := h.subdepartmentUsecase.GetAllSubdepartments()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get subdepartments")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить подотделы",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":           requestID,
		"subdepartments_count": len(subdepartments),
	}).Info("Subdepartments retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"subdepartments": subdepartments,
		"count":          len(subdepartments),
		"request_id":     requestID,
	})
}

// GetSubdepartment handles getting a subdepartment by ID
func (h *SubdepartmentHandler) GetSubdepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": idStr,
			"error":            err.Error(),
		}).Warn("Invalid subdepartment ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID подотдела",
			"request_id": requestID,
		})
		return
	}

	subdepartment, err := h.subdepartmentUsecase.GetSubdepartment(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": id,
			"error":            err.Error(),
		}).Error("Failed to get subdepartment")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось получить подотдел"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Подотдел не найден"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":       requestID,
		"subdepartment_id": id,
	}).Info("Subdepartment retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"subdepartment": subdepartment,
		"request_id":    requestID,
	})
}

// CreateSubdepartment handles creating a new subdepartment
func (h *SubdepartmentHandler) CreateSubdepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.CreateSubdepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid subdepartment creation request")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверные данные запроса",
			"request_id": requestID,
		})
		return
	}

	subdepartment, err := h.subdepartmentUsecase.CreateSubdepartment(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": req.DepartmentID,
			"name":          req.Name,
			"error":         err.Error(),
		}).Error("Failed to create subdepartment")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось создать подотдел"

		if strings.Contains(err.Error(), "validation") || strings.Contains(err.Error(), "required") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":       requestID,
		"subdepartment_id": subdepartment.ID,
		"name":             subdepartment.Name,
		"department_id":    subdepartment.DepartmentID,
	}).Info("Subdepartment created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"subdepartment": subdepartment,
		"request_id":    requestID,
	})
}

// UpdateSubdepartment handles updating an existing subdepartment
func (h *SubdepartmentHandler) UpdateSubdepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": idStr,
			"error":            err.Error(),
		}).Warn("Invalid subdepartment ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID подотдела",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateSubdepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": id,
			"error":            err.Error(),
		}).Warn("Invalid subdepartment update request")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверные данные запроса",
			"request_id": requestID,
		})
		return
	}

	subdepartment, err := h.subdepartmentUsecase.UpdateSubdepartment(uint(id), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": id,
			"error":            err.Error(),
		}).Error("Failed to update subdepartment")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось обновить подотдел"

		if strings.Contains(err.Error(), "validation") || strings.Contains(err.Error(), "required") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":       requestID,
		"subdepartment_id": id,
	}).Info("Subdepartment updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"subdepartment": subdepartment,
		"request_id":    requestID,
	})
}

// DeleteSubdepartment handles deleting a subdepartment
func (h *SubdepartmentHandler) DeleteSubdepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": idStr,
			"error":            err.Error(),
		}).Warn("Invalid subdepartment ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный ID подотдела",
			"request_id": requestID,
		})
		return
	}

	err = h.subdepartmentUsecase.DeleteSubdepartment(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"subdepartment_id": id,
			"error":            err.Error(),
		}).Error("Failed to delete subdepartment")

		statusCode := http.StatusInternalServerError
		errorMessage := "Не удалось удалить подотдел"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Подотдел не найден"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":       requestID,
		"subdepartment_id": id,
	}).Info("Subdepartment deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Subdepartment deleted successfully",
		"request_id": requestID,
	})
}

// ImportSubdepartments handles CSV import of subdepartments
func (h *SubdepartmentHandler) ImportSubdepartments(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get CSV file from form data
	file, err := c.FormFile("file")
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("No file provided for subdepartment import")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Требуется CSV-файл",
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
			"error":      "Не удалось прочитать CSV-файл",
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
			"error":      "Неверный формат CSV: не удалось прочитать заголовок",
			"request_id": requestID,
		})
		return
	}

	// Remove BOM if present
	if len(headers) > 0 && len(headers[0]) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	// Validate required columns
	requiredColumns := []string{"name", "department_id"}
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
			"expected_headers": []string{"name", "department_id"},
			"hint":             "CSV file must have 'name' and 'department_id' columns in the first row",
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

	var successSubdepartments []map[string]string
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

		departmentIDStr := ""
		if idx, ok := headerMap["department_id"]; ok && idx < len(row) {
			departmentIDStr = strings.TrimSpace(row[idx])
		}

		var headID *uint
		if idx, ok := headerMap["head_id"]; ok && idx < len(row) {
			headIDStr := strings.TrimSpace(row[idx])
			if headIDStr != "" {
				id, err := strconv.ParseUint(headIDStr, 10, 32)
				if err != nil {
					errors = append(errors, ImportError{
						Row:     rowNum,
						Name:    name,
						Message: "Invalid head_id: must be a number",
					})
					continue
				}
				headIDUint := uint(id)
				headID = &headIDUint
			}
		}

		// Validate required fields
		if name == "" {
			errors = append(errors, ImportError{
				Row:     rowNum,
				Name:    "",
				Message: "Name is required",
			})
			continue
		}

		if departmentIDStr == "" {
			errors = append(errors, ImportError{
				Row:     rowNum,
				Name:    name,
				Message: "department_id is required",
			})
			continue
		}

		// Parse department_id (hybrid: try numeric first, then by name)
		var departmentID uint
		deptID, err := strconv.ParseUint(departmentIDStr, 10, 32)
		if err == nil {
			// It's a number - verify department exists by ID
			_, err = h.departmentUsecase.GetDepartment(uint(deptID))
			if err != nil {
				errors = append(errors, ImportError{
					Row:     rowNum,
					Name:    name,
					Message: fmt.Sprintf("Department with ID %d not found", deptID),
				})
				continue
			}
			departmentID = uint(deptID)
		} else {
			// Not a number - try to find by name
			dept, err := h.departmentUsecase.GetByName(departmentIDStr)
			if err != nil {
				errors = append(errors, ImportError{
					Row:     rowNum,
					Name:    name,
					Message: fmt.Sprintf("Department '%s' not found (use numeric ID or exact department name)", departmentIDStr),
				})
				continue
			}
			departmentID = dept.ID
		}

		// Create subdepartment request
		req := &models.CreateSubdepartmentRequest{
			Name:         name,
			DepartmentID: departmentID,
			HeadID:       headID,
		}

		// Create subdepartment
		_, err = h.subdepartmentUsecase.CreateSubdepartment(req)
		if err != nil {
			errors = append(errors, ImportError{
				Row:     rowNum,
				Name:    name,
				Message: err.Error(),
			})
			continue
		}

		successSubdepartments = append(successSubdepartments, map[string]string{"name": name})
	}

	totalRows := rowNum - 1 // Exclude header
	successCount := len(successSubdepartments)
	errorCount := len(errors)

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"total_rows":    totalRows,
		"success_count": successCount,
		"error_count":   errorCount,
	}).Info("Subdepartment import completed")

	c.JSON(http.StatusOK, gin.H{
		"total_rows":             totalRows,
		"success_count":          successCount,
		"error_count":            errorCount,
		"success_subdepartments": successSubdepartments,
		"errors":                 errors,
		"request_id":             requestID,
	})
}

// BulkDeleteSubdepartments handles bulk deletion of subdepartments
func (h *SubdepartmentHandler) BulkDeleteSubdepartments(c *gin.Context) {
	requestID := requestid.Get(c)

	var req struct {
		SubdepartmentIDs []uint `json:"subdepartment_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"request_id": requestID,
		})
		return
	}

	if len(req.SubdepartmentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Не указаны ID подотделов",
			"request_id": requestID,
		})
		return
	}

	successCount := 0
	errorCount := 0
	errors := []map[string]interface{}{}

	for _, id := range req.SubdepartmentIDs {
		if err := h.subdepartmentUsecase.DeleteSubdepartment(id); err != nil {
			errorCount++
			errors = append(errors, map[string]interface{}{
				"subdepartment_id": id,
				"message":          err.Error(),
			})
			logger.WithFields(map[string]interface{}{
				"request_id":       requestID,
				"subdepartment_id": id,
				"error":            err.Error(),
			}).Warn("Failed to delete subdepartment in bulk operation")
		} else {
			successCount++
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"total":         len(req.SubdepartmentIDs),
		"success_count": successCount,
		"error_count":   errorCount,
	}).Info("Bulk delete subdepartments completed")

	c.JSON(http.StatusOK, gin.H{
		"success_count": successCount,
		"error_count":   errorCount,
		"errors":        errors,
		"request_id":    requestID,
	})
}
