package seeders

import (
	"fmt"

	"tachyon-messenger/shared/database"
)

// CleanDatabase truncates all tables in the correct order (respecting foreign keys)
func CleanDatabase(db *database.DB) error {
	// Order is important: child tables first, parent tables last
	tables := []string{
		// Calendar
		"event_reminders",
		"event_participants",
		"events",

		// Polls
		"poll_comments",
		"poll_votes",
		"poll_participants",
		"poll_options",
		"polls",

		// Tasks
		"task_checklist_items",
		"task_checklists",
		"task_attachments",
		"task_comments",
		"task_activities",
		"task_assignees",
		"tasks",

		// Chats
		"message_attachments",
		"message_deletions",
		"message_read_receipts",
		"message_reactions",
		"messages",
		"chat_members",
		"chats",

		// Files
		"files",

		// Notifications
		"notification_deliveries",
		"user_notification_preferences",
		"email_templates",
		"notification_templates",
		"notifications",

		// Analytics
		"user_activities",
		"metrics",
		"events", // analytics events (different from calendar events)

		// Users
		"passkeys",
		"two_fas",
		"password_resets",
		"invitations",
		"settings",
		"smtp_settings",
		"users",
		"subdepartments",
		"departments",
	}

	for _, table := range tables {
		if err := db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			// Ignore error if table doesn't exist
			if err.Error() != fmt.Sprintf("ERROR: relation \"%s\" does not exist (SQLSTATE 42P01)", table) {
				fmt.Printf("Warning: failed to truncate %s: %v\n", table, err)
			}
		}
	}

	// Reset sequences
	sequences := []string{
		"departments_id_seq",
		"subdepartments_id_seq",
		"users_id_seq",
		"chats_id_seq",
		"messages_id_seq",
		"tasks_id_seq",
		"polls_id_seq",
		"events_id_seq",
		"files_id_seq",
		"notifications_id_seq",
	}

	for _, seq := range sequences {
		if err := db.DB.Exec(fmt.Sprintf("ALTER SEQUENCE %s RESTART WITH 1", seq)).Error; err != nil {
			// Ignore if sequence doesn't exist
			if err.Error() != fmt.Sprintf("ERROR: relation \"%s\" does not exist (SQLSTATE 42P01)", seq) {
				fmt.Printf("Warning: failed to reset sequence %s: %v\n", seq, err)
			}
		}
	}

	return nil
}
