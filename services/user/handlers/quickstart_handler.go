package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// QuickStartHandler handles quick start import operations
type QuickStartHandler struct {
	departmentUsecase    usecase.DepartmentUsecase
	subdepartmentUsecase usecase.SubdepartmentUsecase
	userUsecase          usecase.UserUsecase
}

// NewQuickStartHandler creates a new quick start handler
func NewQuickStartHandler(
	departmentUsecase usecase.DepartmentUsecase,
	subdepartmentUsecase usecase.SubdepartmentUsecase,
	userUsecase usecase.UserUsecase,
) *QuickStartHandler {
	return &QuickStartHandler{
		departmentUsecase:    departmentUsecase,
		subdepartmentUsecase: subdepartmentUsecase,
		userUsecase:          userUsecase,
	}
}

// ImportQuickStart handles the complete quick start import from ZIP file
// POST /admin/quick-start/import
func (h *QuickStartHandler) ImportQuickStart(c *gin.Context) {
	requestID := requestid.Get(c)
	startTime := time.Now()

	// Get ZIP file from form data
	file, err := c.FormFile("file")
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("No file provided for quick start import")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Требуется ZIP-файл",
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
			"error":      "Не удалось прочитать ZIP-файл",
			"request_id": requestID,
		})
		return
	}
	defer src.Close()

	// Read file into memory
	fileBytes, err := io.ReadAll(src)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to read file bytes")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось прочитать ZIP-файл",
			"request_id": requestID,
		})
		return
	}

	// Open ZIP archive
	zipReader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid ZIP file")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный формат ZIP-файла",
			"request_id": requestID,
		})
		return
	}

	// Extract CSV files from ZIP
	var departmentsCSV, subdepartmentsCSV, usersCSV []byte
	var foundFiles []string
	for _, zipFile := range zipReader.File {
		foundFiles = append(foundFiles, zipFile.Name)

		rc, err := zipFile.Open()
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"filename":   zipFile.Name,
				"error":      err.Error(),
			}).Warn("Failed to open file in ZIP")
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"filename":   zipFile.Name,
				"error":      err.Error(),
			}).Warn("Failed to read file content in ZIP")
			continue
		}

		filename := strings.ToLower(zipFile.Name)
		// Check subdepartments FIRST before departments (because subdepartments contains "departments")
		if strings.Contains(filename, "subdepartments.csv") {
			subdepartmentsCSV = content
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"filename":   zipFile.Name,
				"size":       len(content),
			}).Info("Found subdepartments.csv in ZIP")
		} else if strings.Contains(filename, "departments.csv") {
			departmentsCSV = content
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"filename":   zipFile.Name,
				"size":       len(content),
			}).Info("Found departments.csv in ZIP")
		} else if strings.Contains(filename, "users.csv") {
			usersCSV = content
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"filename":   zipFile.Name,
				"size":       len(content),
			}).Info("Found users.csv in ZIP")
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"files_found":    foundFiles,
		"departments":    len(departmentsCSV),
		"subdepartments": len(subdepartmentsCSV),
		"users":          len(usersCSV),
	}).Info("ZIP extraction completed")

	// Validate required files
	if len(departmentsCSV) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "departments.csv не найден в ZIP-архиве",
			"hint":       "ZIP file must contain departments.csv, subdepartments.csv, and users.csv",
			"request_id": requestID,
		})
		return
	}

	// Initialize response
	response := &models.QuickStartImportResponse{
		Success: true,
		Message: "Quick start import completed",
	}

	// STEP 1: Import Departments
	departmentMap := make(map[string]uint) // name -> ID mapping
	if len(departmentsCSV) > 0 {
		depResult := h.importDepartments(departmentsCSV, requestID)
		response.DepartmentsTotal = depResult.Total
		response.DepartmentsSuccess = depResult.Success
		response.DepartmentsErrors = depResult.Errors
		response.DepartmentsCreated = depResult.Created
		departmentMap = depResult.NameToIDMap
	}

	// STEP 2: Import Subdepartments (if file exists)
	if len(subdepartmentsCSV) > 0 {
		logger.WithFields(map[string]interface{}{
			"request_id":     requestID,
			"csv_size":       len(subdepartmentsCSV),
			"department_map": departmentMap,
		}).Info("Starting subdepartments import step")
		subResult := h.importSubdepartments(subdepartmentsCSV, departmentMap, requestID)
		response.SubdepartmentsTotal = subResult.Total
		response.SubdepartmentsSuccess = subResult.Success
		response.SubdepartmentsErrors = subResult.Errors
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"total":      subResult.Total,
			"success":    subResult.Success,
			"errors":     len(subResult.Errors),
		}).Info("Subdepartments import step completed")
	} else {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
		}).Warn("Subdepartments CSV is empty or not found in ZIP")
	}

	// STEP 3: Import Users (if file exists)
	if len(usersCSV) > 0 {
		userResult := h.importUsers(usersCSV, departmentMap, requestID)
		response.UsersTotal = userResult.Total
		response.UsersSuccess = userResult.Success
		response.UsersErrors = userResult.Errors
	}

	// Calculate totals
	response.TotalRecords = response.DepartmentsTotal + response.SubdepartmentsTotal + response.UsersTotal
	response.TotalSuccess = response.DepartmentsSuccess + response.SubdepartmentsSuccess + response.UsersSuccess
	response.TotalErrors = len(response.DepartmentsErrors) + len(response.SubdepartmentsErrors) + len(response.UsersErrors)
	response.ProcessingTime = int(time.Since(startTime).Milliseconds())

	if response.TotalErrors > 0 {
		response.Success = false
		response.Message = fmt.Sprintf("Import completed with %d errors", response.TotalErrors)
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"total_records": response.TotalRecords,
		"success":       response.TotalSuccess,
		"errors":        response.TotalErrors,
		"duration_ms":   response.ProcessingTime,
	}).Info("Quick start import completed")

	c.JSON(http.StatusOK, response)
}

// departmentImportResult holds the result of department import
type departmentImportResult struct {
	Total       int
	Success     int
	Errors      []models.QuickStartImportError
	Created     []models.QuickStartDepartmentRef
	NameToIDMap map[string]uint
}

// importDepartments processes departments CSV
func (h *QuickStartHandler) importDepartments(csvData []byte, requestID string) departmentImportResult {
	result := departmentImportResult{
		Errors:      []models.QuickStartImportError{},
		Created:     []models.QuickStartDepartmentRef{},
		NameToIDMap: make(map[string]uint),
	}

	// Pre-load all existing departments into the map
	existingDepartments, err := h.departmentUsecase.GetAllDepartments()
	if err == nil {
		for _, dept := range existingDepartments {
			result.NameToIDMap[dept.Name] = dept.ID
		}
		logger.WithFields(map[string]interface{}{
			"count": len(existingDepartments),
		}).Info("Pre-loaded existing departments into map")
	}

	reader := csv.NewReader(bytes.NewReader(csvData))
	headers, err := reader.Read()
	if err != nil {
		result.Errors = append(result.Errors, models.QuickStartImportError{
			Row:      0,
			Message:  "Failed to read CSV headers",
			FileType: "departments",
		})
		return result
	}

	// Remove BOM if present
	if len(headers) > 0 && len(headers[0]) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	// Build header map
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.TrimSpace(strings.ToLower(h))] = i
	}

	// Validate required column
	if _, exists := headerMap["name"]; !exists {
		result.Errors = append(result.Errors, models.QuickStartImportError{
			Row:      0,
			Message:  "Required column 'name' not found",
			FileType: "departments",
		})
		return result
	}

	rowNum := 1
	for {
		rowNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Message:  fmt.Sprintf("Failed to parse row: %v", err),
				FileType: "departments",
			})
			continue
		}

		result.Total++

		// Extract name
		name := ""
		if idx, ok := headerMap["name"]; ok && idx < len(row) {
			name = strings.TrimSpace(row[idx])
		}

		if name == "" {
			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Field:    "name",
				Message:  "Name is required",
				FileType: "departments",
			})
			continue
		}

		// Check if department already exists in map (count as success)
		if existingID, exists := result.NameToIDMap[name]; exists {
			logger.WithFields(map[string]interface{}{
				"name": name,
				"id":   existingID,
			}).Debug("Department already exists (active), counting as success")
			// Count as success since it's already there and active
			result.Success++
			result.Created = append(result.Created, models.QuickStartDepartmentRef{
				ID:   existingID,
				Name: name,
			})
			continue
		}

		// Try to create new department
		dept, err := h.departmentUsecase.CreateDepartment(&models.CreateDepartmentRequest{
			Name: name,
		})
		if err != nil {
			// If creation failed due to duplicate name, check if it's a soft-deleted department
			if strings.Contains(strings.ToLower(err.Error()), "already exists") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				// Try to find and restore the soft-deleted department
				existingDept, getErr := h.departmentUsecase.GetDepartmentByNameIncludingDeleted(name)
				if getErr == nil && existingDept != nil && existingDept.DeletedAt.Valid {
					// Restore the soft-deleted department
					if restoreErr := h.departmentUsecase.RestoreDepartment(existingDept.ID); restoreErr == nil {
						// Add restored department to the map
						result.NameToIDMap[existingDept.Name] = existingDept.ID
						result.Success++
						result.Created = append(result.Created, models.QuickStartDepartmentRef{
							ID:   existingDept.ID,
							Name: existingDept.Name,
						})
						logger.WithFields(map[string]interface{}{
							"name": existingDept.Name,
							"id":   existingDept.ID,
						}).Info("Restored soft-deleted department")
						continue
					}
				}
			}

			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Value:    name,
				Message:  err.Error(),
				FileType: "departments",
			})
			continue
		}

		result.Success++
		result.Created = append(result.Created, models.QuickStartDepartmentRef{
			ID:   dept.ID,
			Name: dept.Name,
		})
		result.NameToIDMap[dept.Name] = dept.ID
	}

	return result
}

// subdepartmentImportResult holds the result of subdepartment import
type subdepartmentImportResult struct {
	Total   int
	Success int
	Errors  []models.QuickStartImportError
}

// importSubdepartments processes subdepartments CSV
func (h *QuickStartHandler) importSubdepartments(csvData []byte, departmentMap map[string]uint, requestID string) subdepartmentImportResult {
	result := subdepartmentImportResult{
		Errors: []models.QuickStartImportError{},
	}

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"csv_size":       len(csvData),
		"department_map": departmentMap,
	}).Info("Starting subdepartment import")

	reader := csv.NewReader(bytes.NewReader(csvData))
	headers, err := reader.Read()
	if err != nil {
		result.Errors = append(result.Errors, models.QuickStartImportError{
			Row:      0,
			Message:  "Failed to read CSV headers",
			FileType: "subdepartments",
		})
		return result
	}

	// Remove BOM
	if len(headers) > 0 && len(headers[0]) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.TrimSpace(strings.ToLower(h))] = i
	}

	rowNum := 1
	for {
		rowNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		result.Total++

		name := ""
		if idx, ok := headerMap["name"]; ok && idx < len(row) {
			name = strings.TrimSpace(row[idx])
		}

		// Try to get department by name or ID
		var departmentID uint
		if idx, ok := headerMap["department_name"]; ok && idx < len(row) {
			deptName := strings.TrimSpace(row[idx])
			logger.WithFields(map[string]interface{}{
				"row":             rowNum,
				"department_name": deptName,
				"available_depts": departmentMap,
			}).Debug("Looking up department for subdepartment")
			if id, exists := departmentMap[deptName]; exists {
				departmentID = id
			} else {
				logger.WithFields(map[string]interface{}{
					"row":             rowNum,
					"department_name": deptName,
				}).Warn("Department not found for subdepartment")
			}
		}

		// Fallback to department_id
		if departmentID == 0 {
			if idx, ok := headerMap["department_id"]; ok && idx < len(row) {
				if id, err := strconv.ParseUint(strings.TrimSpace(row[idx]), 10, 32); err == nil {
					departmentID = uint(id)
				}
			}
		}

		if name == "" || departmentID == 0 {
			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Message:  "Name and department are required",
				FileType: "subdepartments",
			})
			continue
		}

		_, err = h.subdepartmentUsecase.CreateSubdepartment(&models.CreateSubdepartmentRequest{
			Name:         name,
			DepartmentID: departmentID,
		})
		if err != nil {
			// If creation failed due to duplicate, try to restore soft-deleted subdepartment
			if strings.Contains(strings.ToLower(err.Error()), "already exists") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				// Try to find and restore the soft-deleted subdepartment
				existingSub, getErr := h.subdepartmentUsecase.GetSubdepartmentByNameAndDepartmentIncludingDeleted(name, departmentID)
				if getErr == nil && existingSub != nil && existingSub.DeletedAt.Valid {
					// Restore the soft-deleted subdepartment
					if restoreErr := h.subdepartmentUsecase.RestoreSubdepartment(existingSub.ID); restoreErr == nil {
						result.Success++
						logger.WithFields(map[string]interface{}{
							"name":          name,
							"id":            existingSub.ID,
							"department_id": departmentID,
						}).Info("Restored soft-deleted subdepartment")
						continue
					}
				}
			}

			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Value:    name,
				Message:  err.Error(),
				FileType: "subdepartments",
			})
			continue
		}

		result.Success++
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"total":      result.Total,
		"success":    result.Success,
		"errors":     len(result.Errors),
	}).Info("Subdepartment import completed")

	return result
}

// userImportResult holds the result of user import
type userImportResult struct {
	Total   int
	Success int
	Errors  []models.QuickStartImportError
}

// importUsers processes users CSV
func (h *QuickStartHandler) importUsers(csvData []byte, departmentMap map[string]uint, requestID string) userImportResult {
	result := userImportResult{
		Errors: []models.QuickStartImportError{},
	}

	reader := csv.NewReader(bytes.NewReader(csvData))
	headers, err := reader.Read()
	if err != nil {
		result.Errors = append(result.Errors, models.QuickStartImportError{
			Row:      0,
			Message:  "Failed to read CSV headers",
			FileType: "users",
		})
		return result
	}

	// Remove BOM
	if len(headers) > 0 && len(headers[0]) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.TrimSpace(strings.ToLower(h))] = i
	}

	rowNum := 1
	for {
		rowNum++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		result.Total++

		// Extract fields
		email := h.getCSVField(row, headerMap, "email")
		name := h.getCSVField(row, headerMap, "name")
		firstName := h.getCSVField(row, headerMap, "first_name")
		lastName := h.getCSVField(row, headerMap, "last_name")
		middleName := h.getCSVField(row, headerMap, "middle_name")
		role := h.getCSVField(row, headerMap, "role")
		phone := h.getCSVField(row, headerMap, "phone")
		position := h.getCSVField(row, headerMap, "position")

		if email == "" || name == "" || firstName == "" || lastName == "" {
			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Message:  "Email, name, first_name, and last_name are required",
				FileType: "users",
			})
			continue
		}

		// Get department ID
		var departmentID *uint
		deptName := h.getCSVField(row, headerMap, "department_name")
		if deptName != "" {
			if id, exists := departmentMap[deptName]; exists {
				departmentID = &id
			}
		}

		userReq := &models.CreateUserRequest{
			Email:        email,
			Name:         name,
			FirstName:    firstName,
			LastName:     lastName,
			MiddleName:   middleName,
			Role:         role,
			DepartmentID: departmentID,
			Phone:        phone,
			Position:     position,
		}

		_, err = h.userUsecase.CreateUser(userReq)
		if err != nil {
			result.Errors = append(result.Errors, models.QuickStartImportError{
				Row:      rowNum,
				Value:    email,
				Message:  err.Error(),
				FileType: "users",
			})
			continue
		}

		result.Success++
	}

	return result
}

// getCSVField is a helper to safely get field from CSV row
func (h *QuickStartHandler) getCSVField(row []string, headerMap map[string]int, field string) string {
	if idx, ok := headerMap[field]; ok && idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}
