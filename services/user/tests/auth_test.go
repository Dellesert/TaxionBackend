package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"tachyon-messenger/services/user/handlers"
	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB() (*database.DB, error) {
	// Use in-memory SQLite for testing
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db := &database.DB{DB: gormDB}

	// Run migrations
	err = db.AutoMigrate(&models.Department{}, &models.User{})
	if err != nil {
		return nil, err
	}

	// Create test departments
	departments := []models.Department{
		{Name: "Engineering"},
		{Name: "Marketing"},
		{Name: "HR"},
	}

	for _, dept := range departments {
		db.Create(&dept)
	}

	return db, nil
}

// setupTestRouter creates a test router with all handlers
func setupTestRouter() (*gin.Engine, error) {
	gin.SetMode(gin.TestMode)

	// Setup test database
	db, err := setupTestDB()
	if err != nil {
		return nil, err
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	departmentRepo := repository.NewDepartmentRepository(db)

	// Create JWT config for testing
	jwtConfig := &middleware.JWTConfig{
		Secret:               "test-secret-key",
		AccessTokenDuration:  middleware.DefaultJWTConfig("").AccessTokenDuration,
		RefreshTokenDuration: middleware.DefaultJWTConfig("").RefreshTokenDuration,
		Issuer:               "test-tachyon-messenger",
	}

	// Initialize usecases
	userUsecase := usecase.NewUserUsecase(userRepo)
	authUsecase := usecase.NewAuthUsecase(userRepo, departmentRepo, jwtConfig)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userUsecase)
	authHandler := handlers.NewAuthHandler(authUsecase)

	// Setup router
	router := gin.New()

	// Add routes
	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/login", authHandler.Login)
	router.GET("/api/v1/users/:id", middleware.JWTMiddleware(jwtConfig), userHandler.GetUser)

	return router, nil
}

func TestUserRegistration(t *testing.T) {
	router, err := setupTestRouter()
	require.NoError(t, err)

	tests := []struct {
		name           string
		payload        models.CreateUserRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful registration",
			payload: models.CreateUserRequest{
				Email:    "test@example.com",
				Name:     "Test User",
				Password: "password123",
				Role:     "employee",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "registration with department",
			payload: models.CreateUserRequest{
				Email:        "test2@example.com",
				Name:         "Test User 2",
				Password:     "password123",
				Role:         "employee",
				DepartmentID: func() *uint { id := uint(1); return &id }(),
				Position:     "Software Engineer",
				Phone:        "+1234567890",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "duplicate email",
			payload: models.CreateUserRequest{
				Email:    "test@example.com", // Same as first test
				Name:     "Another User",
				Password: "password123",
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "already exists",
		},
		{
			name: "invalid email",
			payload: models.CreateUserRequest{
				Email:    "invalid-email",
				Name:     "Test User",
				Password: "password123",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "weak password",
			payload: models.CreateUserRequest{
				Email:    "test3@example.com",
				Name:     "Test User",
				Password: "123", // Too short
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing required fields",
			payload: models.CreateUserRequest{
				Email: "test4@example.com",
				// Missing name and password
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedStatus == http.StatusCreated {
				// Check successful response structure
				assert.Contains(t, response, "user")
				assert.Contains(t, response, "message")

				user := response["user"].(map[string]interface{})
				assert.Equal(t, tt.payload.Email, user["email"])
				assert.Equal(t, tt.payload.Name, user["name"])
				assert.NotContains(t, user, "hashed_password") // Password should not be in response
			} else {
				// Check error response
				assert.Contains(t, response, "error")
				if tt.expectedError != "" {
					assert.Contains(t, response["error"].(string), tt.expectedError)
				}
			}
		})
	}
}

func TestUserLogin(t *testing.T) {
	router, err := setupTestRouter()
	require.NoError(t, err)

	// First, register a user
	registerPayload := models.CreateUserRequest{
		Email:    "login-test@example.com",
		Name:     "Login Test User",
		Password: "password123",
		Role:     "employee",
	}

	payloadBytes, err := json.Marshal(registerPayload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Now test login scenarios
	tests := []struct {
		name           string
		email          string
		password       string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful login",
			email:          "login-test@example.com",
			password:       "password123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "wrong password",
			email:          "login-test@example.com",
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid email or password",
		},
		{
			name:           "non-existent user",
			email:          "nonexistent@example.com",
			password:       "password123",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid email or password",
		},
		{
			name:           "missing email",
			email:          "",
			password:       "password123",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing password",
			email:          "login-test@example.com",
			password:       "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create login payload
			loginPayload := map[string]string{
				"email":    tt.email,
				"password": tt.password,
			}

			payloadBytes, err := json.Marshal(loginPayload)
			require.NoError(t, err)

			// Create request
			req, err := http.NewRequest("POST", "/auth/login", bytes.NewBuffer(payloadBytes))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Perform request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedStatus == http.StatusOK {
				// Check successful login response
				assert.Contains(t, response, "user")
				assert.Contains(t, response, "tokens")
				assert.Contains(t, response, "message")

				user := response["user"].(map[string]interface{})
				assert.Equal(t, tt.email, user["email"])

				tokens := response["tokens"].(map[string]interface{})
				assert.Contains(t, tokens, "access_token")
				assert.Contains(t, tokens, "refresh_token")
				assert.NotEmpty(t, tokens["access_token"])
				assert.NotEmpty(t, tokens["refresh_token"])
			} else {
				// Check error response
				assert.Contains(t, response, "error")
				if tt.expectedError != "" {
					assert.Contains(t, response["error"].(string), tt.expectedError)
				}
			}
		})
	}
}

func TestJWTAuthentication(t *testing.T) {
	router, err := setupTestRouter()
	require.NoError(t, err)

	// Register and login to get a token
	registerPayload := models.CreateUserRequest{
		Email:    "jwt-test@example.com",
		Name:     "JWT Test User",
		Password: "password123",
	}

	// Register user
	payloadBytes, err := json.Marshal(registerPayload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var registerResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &registerResponse)
	require.NoError(t, err)

	userID := registerResponse["user"].(map[string]interface{})["id"].(float64)

	// Login to get token
	loginPayload := map[string]string{
		"email":    registerPayload.Email,
		"password": registerPayload.Password,
	}

	payloadBytes, err = json.Marshal(loginPayload)
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/auth/login", bytes.NewBuffer(payloadBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var loginResponse map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &loginResponse)
	require.NoError(t, err)

	tokens := loginResponse["tokens"].(map[string]interface{})
	accessToken := tokens["access_token"].(string)

	// Test protected endpoint with valid token
	req, err = http.NewRequest("GET", fmt.Sprintf("/api/v1/users/%.0f", userID), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test protected endpoint without token
	req, err = http.NewRequest("GET", fmt.Sprintf("/api/v1/users/%.0f", userID), nil)
	require.NoError(t, err)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test protected endpoint with invalid token
	req, err = http.NewRequest("GET", fmt.Sprintf("/api/v1/users/%.0f", userID), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
