// Создайте файл: services/chat/cmd/migrate/main.go
package main

import (
	"flag"
	"fmt"

	"tachyon-messenger/services/chat/migrations"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
)

func main() {
	var (
		action = flag.String("action", "up", "Migration action: up, status, down")
		help   = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Initialize logger
	log := logger.New(&logger.Config{
		Level:       "info",
		Format:      "text",
		Environment: "development",
	})

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create migration manager
	manager := migrations.NewMigrationManager(db, log)

	switch *action {
	case "up":
		log.Info("Running migrations...")
		if err := manager.RunMigrations(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Info("✅ Migrations completed successfully!")

	case "status":
		log.Info("Checking migration status...")
		applied, err := manager.GetAppliedMigrations()
		if err != nil {
			log.Fatalf("Failed to get migration status: %v", err)
		}

		if len(applied) == 0 {
			log.Info("No migrations have been applied")
		} else {
			log.Info("Applied migrations:")
			for _, migration := range applied {
				log.Infof("  ✓ %s - %s (applied: %s)", migration.Version, migration.Name, *migration.AppliedAt)
			}
		}

	default:
		log.Fatalf("Unknown action: %s. Use 'up' or 'status'", *action)
	}
}

func showHelp() {
	fmt.Println("Chat Service Migration Tool")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  go run services/chat/cmd/migrate/main.go [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -action string")
	fmt.Println("        Migration action: up, status (default \"up\")")
	fmt.Println("  -help")
	fmt.Println("        Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  go run services/chat/cmd/migrate/main.go -action=up")
	fmt.Println("  go run services/chat/cmd/migrate/main.go -action=status")
}
