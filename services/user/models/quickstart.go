package models

// QuickStartImportRequest represents the request for quick start import
type QuickStartImportRequest struct {
	ClearExistingData bool `json:"clear_existing_data"` // Optional: clear all data before import
}

// QuickStartImportResponse represents the response from quick start import
type QuickStartImportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`

	// Departments import results
	DepartmentsTotal   int                       `json:"departments_total"`
	DepartmentsSuccess int                       `json:"departments_success"`
	DepartmentsErrors  []QuickStartImportError   `json:"departments_errors,omitempty"`
	DepartmentsCreated []QuickStartDepartmentRef `json:"departments_created,omitempty"`

	// Subdepartments import results
	SubdepartmentsTotal   int                     `json:"subdepartments_total"`
	SubdepartmentsSuccess int                     `json:"subdepartments_success"`
	SubdepartmentsErrors  []QuickStartImportError `json:"subdepartments_errors,omitempty"`

	// Users import results
	UsersTotal   int                     `json:"users_total"`
	UsersSuccess int                     `json:"users_success"`
	UsersErrors  []QuickStartImportError `json:"users_errors,omitempty"`

	// Overall statistics
	TotalRecords   int `json:"total_records"`
	TotalSuccess   int `json:"total_success"`
	TotalErrors    int `json:"total_errors"`
	ProcessingTime int `json:"processing_time_ms"`
}

// QuickStartImportError represents an error during quick start import
type QuickStartImportError struct {
	Row      int    `json:"row"`
	Field    string `json:"field,omitempty"`
	Value    string `json:"value,omitempty"`
	Message  string `json:"message"`
	FileType string `json:"file_type"` // "departments", "subdepartments", "users"
}

// QuickStartDepartmentRef represents a reference to created department
type QuickStartDepartmentRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// QuickStartProgress represents the current progress of import
type QuickStartProgress struct {
	Stage            string  `json:"stage"` // "extracting", "departments", "subdepartments", "users", "completed"
	CurrentFile      string  `json:"current_file,omitempty"`
	ProcessedRecords int     `json:"processed_records"`
	TotalRecords     int     `json:"total_records"`
	PercentComplete  float64 `json:"percent_complete"`
	Message          string  `json:"message"`
}
