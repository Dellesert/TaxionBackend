package tests

import (
	"fmt"
	"os"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	sharedmodels "tachyon-messenger/shared/models"
)

func RunDatabaseCheck() {
	// Initialize logger
	logger := logger.New(&logger.Config{
		Level:       "info",
		Format:      "text",
		Environment: "development",
	})

	logger.Info("Starting database connection test...")

	// Set default environment variables if not set
	setDefaultEnvVars()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Infof("Database URL: %s", maskDatabaseURL(cfg.Database.URL))

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	logger.Info("✓ Successfully connected to database!")

	// Test database health
	if err := db.Health(); err != nil {
		logger.Fatalf("Database health check failed: %v", err)
	}

	logger.Info("✓ Database health check passed!")

	// Get database statistics
	stats, err := db.Stats()
	if err != nil {
		logger.Warnf("Failed to get database stats: %v", err)
	} else {
		logger.Infof("Database connection stats:")
		for key, value := range stats {
			logger.Infof("  %s: %v", key, value)
		}
	}

	// Run migrations
	logger.Info("Running database migrations...")
	if err := db.Migrate(&models.Department{}, &models.User{}); err != nil {
		logger.Fatalf("Failed to run migrations: %v", err)
	}

	logger.Info("✓ Migrations completed successfully!")

	// Check if tables exist and their structure
	logger.Info("Checking table structures...")
	checkTableStructure(db, logger)

	// Create sample departments if they don't exist
	logger.Info("Creating sample departments...")
	createSampleDepartments(db, logger)

	// Verify departments
	logger.Info("Verifying departments...")
	verifyDepartments(db, logger)

	// Test database operations
	testDatabaseOperations(db, logger)

	logger.Info("✅ Database check completed successfully!")
}

func setDefaultEnvVars() {
	// Set default environment variables if not already set
	defaults := map[string]string{
		"DATABASE_URL": "postgres://tachyon_user:tachyon_password@localhost:5432/tachyon_messenger?sslmode=disable",
		"REDIS_URL":    "redis://localhost:6379",
		"JWT_SECRET":   "your-super-secret-jwt-key-here",
		"SERVER_PORT":  "8081",
	}

	for key, defaultValue := range defaults {
		if os.Getenv(key) == "" {
			os.Setenv(key, defaultValue)
			fmt.Printf("Set default %s\n", key)
		}
	}
}

func maskDatabaseURL(url string) string {
	// Simple masking for security - hide password in logs
	if len(url) < 20 {
		return "****"
	}

	// Find password part (between : and @)
	parts := strings.Split(url, ":")
	if len(parts) >= 3 {
		// Replace password part
		passwordEnd := strings.Index(parts[2], "@")
		if passwordEnd > 0 {
			parts[2] = "****" + parts[2][passwordEnd:]
		}
		return strings.Join(parts, ":")
	}

	return url[:15] + "****" + url[len(url)-10:]
}

func checkTableStructure(db *database.DB, logger *logger.Logger) {
	// Check if tables exist by trying to query them
	var count int64

	// Check departments table
	if err := db.Model(&models.Department{}).Count(&count).Error; err != nil {
		logger.Errorf("❌ departments table issue: %v", err)
	} else {
		logger.Infof("✓ departments table exists (count: %d)", count)
	}

	// Check users table
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		logger.Errorf("❌ users table issue: %v", err)
	} else {
		logger.Infof("✓ users table exists (count: %d)", count)
	}

	// Test relationships by trying to load a user with department
	var user models.User
	err := db.Preload("Department").First(&user).Error
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			logger.Info("ℹ No users found yet (table is empty)")
		} else {
			logger.Warnf("⚠ User-Department relationship test failed: %v", err)
		}
	} else {
		logger.Info("✓ User-Department relationship working")
		if user.Department != nil {
			logger.Infof("  Found user '%s' in department '%s'", user.Name, user.Department.Name)
		}
	}

	// Check table columns by describing structure
	logger.Info("Checking table columns...")
	checkTableColumns(db, logger)
}

func checkTableColumns(db *database.DB, logger *logger.Logger) {
	// Get underlying SQL DB to check table structure
	sqlDB, err := db.DB.DB()
	if err != nil {
		logger.Warnf("Could not get SQL DB instance: %v", err)
		return
	}

	// Check departments table columns
	rows, err := sqlDB.Query(`
		SELECT column_name, data_type, is_nullable, column_default 
		FROM information_schema.columns 
		WHERE table_name = 'departments' 
		ORDER BY ordinal_position
	`)
	if err != nil {
		logger.Warnf("Could not check departments table structure: %v", err)
	} else {
		logger.Info("Departments table structure:")
		for rows.Next() {
			var colName, dataType, nullable, defaultVal string
			if err := rows.Scan(&colName, &dataType, &nullable, &defaultVal); err == nil {
				logger.Infof("  - %s: %s (nullable: %s, default: %s)",
					colName, dataType, nullable, defaultVal)
			}
		}
		rows.Close()
	}

	// Check users table columns
	rows, err = sqlDB.Query(`
		SELECT column_name, data_type, is_nullable, column_default 
		FROM information_schema.columns 
		WHERE table_name = 'users' 
		ORDER BY ordinal_position
	`)
	if err != nil {
		logger.Warnf("Could not check users table structure: %v", err)
	} else {
		logger.Info("Users table structure:")
		for rows.Next() {
			var colName, dataType, nullable, defaultVal string
			if err := rows.Scan(&colName, &dataType, &nullable, &defaultVal); err == nil {
				logger.Infof("  - %s: %s (nullable: %s, default: %s)",
					colName, dataType, nullable, defaultVal)
			}
		}
		rows.Close()
	}
}

func createSampleDepartments(db *database.DB, logger *logger.Logger) {
	departments := []models.Department{
		{Name: "Engineering"},
		{Name: "Marketing"},
		{Name: "Sales"},
		{Name: "HR"},
		{Name: "Finance"},
		{Name: "Operations"},
		{Name: "Support"},
		{Name: "Design"},
	}

	for _, dept := range departments {
		// Check if department already exists
		var existing models.Department
		result := db.Where("name = ?", dept.Name).First(&existing)

		if result.Error != nil {
			// Department doesn't exist, create it
			if err := db.Create(&dept).Error; err != nil {
				logger.Errorf("❌ Error creating department %s: %v", dept.Name, err)
			} else {
				logger.Infof("✓ Created department: %s (ID: %d)", dept.Name, dept.ID)
			}
		} else {
			logger.Infof("- Department already exists: %s (ID: %d)", existing.Name, existing.ID)
		}
	}
}

func verifyDepartments(db *database.DB, logger *logger.Logger) {
	var departments []models.Department
	if err := db.Order("name ASC").Find(&departments).Error; err != nil {
		logger.Errorf("❌ Failed to fetch departments: %v", err)
		return
	}

	logger.Infof("Found %d departments:", len(departments))
	for _, dept := range departments {
		logger.Infof("  - ID: %d, Name: %s, Created: %v",
			dept.ID, dept.Name, dept.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func testDatabaseOperations(db *database.DB, logger *logger.Logger) {
	logger.Info("Testing database operations...")

	// Test department creation
	testDept := &models.Department{
		Name: "Test Department " + fmt.Sprintf("%d", os.Getpid()),
	}

	if err := db.Create(testDept).Error; err != nil {
		logger.Errorf("❌ Failed to create test department: %v", err)
		return
	}

	logger.Infof("✓ Created test department '%s' with ID: %d", testDept.Name, testDept.ID)

	// Test user creation
	testUser := &models.User{
		Email:          fmt.Sprintf("test-%d@example.com", os.Getpid()),
		Name:           "Test User",
		HashedPassword: "$2a$10$example.hash.here", // Example bcrypt hash
		Role:           sharedmodels.RoleEmployee,
		Status:         sharedmodels.StatusOffline,
		DepartmentID:   &testDept.ID,
		Position:       "Test Engineer",
		IsActive:       true,
	}

	if err := db.Create(testUser).Error; err != nil {
		logger.Errorf("❌ Failed to create test user: %v", err)
	} else {
		logger.Infof("✓ Created test user '%s' with ID: %d", testUser.Name, testUser.ID)

		// Test user retrieval with department
		var retrievedUser models.User
		if err := db.Preload("Department").First(&retrievedUser, testUser.ID).Error; err != nil {
			logger.Errorf("❌ Failed to retrieve test user: %v", err)
		} else {
			logger.Infof("✓ Retrieved user: %s (%s)", retrievedUser.Name, retrievedUser.Email)
			logger.Infof("  - Role: %s, Status: %s, Active: %v",
				retrievedUser.Role, retrievedUser.Status, retrievedUser.IsActive)
			if retrievedUser.Department != nil {
				logger.Infof("  - Department: %s (ID: %d)",
					retrievedUser.Department.Name, retrievedUser.Department.ID)
			}
		}

		// Test user update
		testUser.Status = sharedmodels.StatusOnline
		testUser.Position = "Senior Test Engineer"
		if err := db.Save(testUser).Error; err != nil {
			logger.Errorf("❌ Failed to update test user: %v", err)
		} else {
			logger.Info("✓ Updated test user successfully")
		}

		// Test user query with filters
		var activeUsers []models.User
		if err := db.Where("is_active = ?", true).Find(&activeUsers).Error; err != nil {
			logger.Errorf("❌ Failed to query active users: %v", err)
		} else {
			logger.Infof("✓ Found %d active users", len(activeUsers))
		}
	}

	// Test foreign key constraints
	logger.Info("Testing foreign key constraints...")

	// Try to create user with invalid department ID
	invalidUser := &models.User{
		Email:          fmt.Sprintf("invalid-%d@example.com", os.Getpid()),
		Name:           "Invalid User",
		HashedPassword: "$2a$10$example.hash.here",
		DepartmentID:   func() *uint { id := uint(99999); return &id }(), // Non-existent department
	}

	if err := db.Create(invalidUser).Error; err != nil {
		logger.Info("✓ Foreign key constraint working (rejected invalid department_id)")
	} else {
		logger.Warn("⚠ Foreign key constraint not working as expected")
		// Clean up
		db.Delete(invalidUser)
	}

	// Clean up test data
	logger.Info("Cleaning up test data...")
	if testUser.ID > 0 {
		if err := db.Delete(testUser).Error; err != nil {
			logger.Warnf("Failed to delete test user: %v", err)
		} else {
			logger.Info("✓ Deleted test user")
		}
	}

	if testDept.ID > 0 {
		if err := db.Delete(testDept).Error; err != nil {
			logger.Warnf("Failed to delete test department: %v", err)
		} else {
			logger.Info("✓ Deleted test department")
		}
	}

	logger.Info("✓ Database operations test completed")
}
