package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"tachyon-messenger/scripts/seed/seeders"
	"tachyon-messenger/shared/database"
)

type Config struct {
	Clean       bool
	Users       bool
	Chats       bool
	Tasks       bool
	Polls       bool
	Calendar    bool
	All         bool
	UserCount   int
	ChatCount   int
	TaskCount   int
	PollCount   int
	EventCount  int
}

func main() {
	config := parseFlags()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Connect to database
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	dbConfig := database.ConfigFromEnv(dsn)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("🌱 Starting database seeding...")

	// Clean database if requested
	if config.Clean {
		fmt.Println("🧹 Cleaning database...")
		if err := seeders.CleanDatabase(db); err != nil {
			log.Fatalf("Failed to clean database: %v", err)
		}
		fmt.Println("✅ Database cleaned")
	}

	// Seed users and departments (required for other seeders)
	if config.All || config.Users || config.Chats || config.Tasks || config.Polls || config.Calendar {
		fmt.Printf("👥 Seeding users and departments (%d users)...\n", config.UserCount)
		users, departments, err := seeders.SeedUsers(db, config.UserCount)
		if err != nil {
			log.Fatalf("Failed to seed users: %v", err)
		}
		fmt.Printf("✅ Created %d users in %d departments\n", len(users), len(departments))

		// Store users for other seeders
		seeders.SetUsers(users)
		seeders.SetDepartments(departments)
	}

	// Seed chats
	if config.All || config.Chats {
		fmt.Printf("💬 Seeding chats (%d chats)...\n", config.ChatCount)
		chats, err := seeders.SeedChats(db, config.ChatCount)
		if err != nil {
			log.Fatalf("Failed to seed chats: %v", err)
		}
		fmt.Printf("✅ Created %d chats with messages\n", len(chats))
	}

	// Seed tasks
	if config.All || config.Tasks {
		fmt.Printf("📋 Seeding tasks (%d tasks)...\n", config.TaskCount)
		tasks, err := seeders.SeedTasks(db, config.TaskCount)
		if err != nil {
			log.Fatalf("Failed to seed tasks: %v", err)
		}
		fmt.Printf("✅ Created %d tasks with subtasks and comments\n", len(tasks))
	}

	// Seed polls
	if config.All || config.Polls {
		fmt.Printf("📊 Seeding polls (%d polls)...\n", config.PollCount)
		polls, err := seeders.SeedPolls(db, config.PollCount)
		if err != nil {
			log.Fatalf("Failed to seed polls: %v", err)
		}
		fmt.Printf("✅ Created %d polls with votes\n", len(polls))
	}

	// Seed calendar events
	if config.All || config.Calendar {
		fmt.Printf("📅 Seeding calendar events (%d events)...\n", config.EventCount)
		events, err := seeders.SeedCalendar(db, config.EventCount)
		if err != nil {
			log.Fatalf("Failed to seed calendar: %v", err)
		}
		fmt.Printf("✅ Created %d calendar events\n", len(events))
	}

	fmt.Println("\n🎉 Database seeding completed successfully!")
}

func parseFlags() *Config {
	config := &Config{}

	flag.BoolVar(&config.Clean, "clean", false, "Clean database before seeding")
	flag.BoolVar(&config.All, "all", false, "Seed all data")
	flag.BoolVar(&config.Users, "users", false, "Seed users and departments")
	flag.BoolVar(&config.Chats, "chats", false, "Seed chats and messages")
	flag.BoolVar(&config.Tasks, "tasks", false, "Seed tasks")
	flag.BoolVar(&config.Polls, "polls", false, "Seed polls")
	flag.BoolVar(&config.Calendar, "calendar", false, "Seed calendar events")

	flag.IntVar(&config.UserCount, "user-count", 50, "Number of users to create")
	flag.IntVar(&config.ChatCount, "chat-count", 30, "Number of chats to create")
	flag.IntVar(&config.TaskCount, "task-count", 100, "Number of tasks to create")
	flag.IntVar(&config.PollCount, "poll-count", 20, "Number of polls to create")
	flag.IntVar(&config.EventCount, "event-count", 80, "Number of calendar events to create")

	flag.Parse()

	// If no specific flag is set, enable all
	if !config.Users && !config.Chats && !config.Tasks && !config.Polls && !config.Calendar {
		config.All = true
	}

	return config
}
