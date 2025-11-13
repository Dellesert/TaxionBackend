package errors

import (
	"github.com/gin-gonic/gin"
)

// RespondWithError sends an API error as JSON response
func RespondWithError(c *gin.Context, err *APIError) {
	c.JSON(err.StatusCode, err)
}

// RespondWithFieldErrors sends validation errors as JSON response
func RespondWithFieldErrors(c *gin.Context, requestID string, fields []FieldError) {
	err := NewAPIError(400, ValidationFailed, "Validation failed").
		WithRequestID(requestID)
	err.Fields = fields
	c.JSON(err.StatusCode, err)
}
