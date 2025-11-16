package migrations

import (
	"fmt"
	"strings"

	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// containsStr checks if a string contains a substring (case-insensitive)
func containsStr(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// RunMigrations runs all database migrations
func RunMigrations(db *database.DB) error {
	fmt.Println("🔄 Starting custom SQL migrations...")

	// Run custom SQL migrations before GORM auto-migrate
	if err := migrateTasksTable(db); err != nil {
		return fmt.Errorf("failed to migrate tasks table: %w", err)
	}

	fmt.Println("✅ Custom SQL migrations completed successfully")
	return nil
}

// migrateTasksTable migrates existing tasks table to new structure
func migrateTasksTable(db *database.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		fmt.Println("📋 Checking if tasks table needs migration...")

		// Check if old 'created_by' column exists
		var columnExists bool
		err := tx.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'tasks'
				AND column_name = 'created_by'
			)
		`).Scan(&columnExists).Error
		if err != nil {
			return fmt.Errorf("failed to check column existence: %w", err)
		}

		// If old column doesn't exist, this is a fresh installation - skip migration
		if !columnExists {
			fmt.Println("✓ Fresh installation detected, no migration needed")
			return nil
		}

		fmt.Println("📊 Existing tasks table found, migrating data...")

		// Check if new column already exists
		var newColumnExists bool
		err = tx.Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'tasks'
				AND column_name = 'created_by_user_id'
			)
		`).Scan(&newColumnExists).Error
		if err != nil {
			return fmt.Errorf("failed to check new column existence: %w", err)
		}

		// If new column already exists and is NOT NULL, migration already done
		if newColumnExists {
			var isNotNull bool
			err = tx.Raw(`
				SELECT is_nullable = 'NO'
				FROM information_schema.columns
				WHERE table_name = 'tasks'
				AND column_name = 'created_by_user_id'
			`).Scan(&isNotNull).Error
			if err != nil {
				return fmt.Errorf("failed to check column nullability: %w", err)
			}
			if isNotNull {
				fmt.Println("✓ Migration already completed")
				return nil
			}
		}

		fmt.Println("🔧 Adding new columns...")

		// Step 1: Add new columns as nullable
		migrations := []string{
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS created_by_user_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_to_user_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS delegated_from_user_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS original_assignee_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_to_department_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS parent_task_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS progress_percentage INTEGER DEFAULT 0`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS first_viewed_at TIMESTAMP`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS first_viewed_by_user_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_status_changed_by_user_id INTEGER`,
			`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP`,
		}

		for _, migration := range migrations {
			if err := tx.Exec(migration).Error; err != nil {
				return fmt.Errorf("failed to add column: %w", err)
			}
		}

		fmt.Println("✓ New columns added")
		fmt.Println("📦 Migrating data from old columns...")
		dataMigrations := []string{
			`UPDATE tasks SET created_by_user_id = created_by WHERE created_by IS NOT NULL AND created_by_user_id IS NULL`,
			`UPDATE tasks SET assigned_to_user_id = assigned_to WHERE assigned_to IS NOT NULL AND assigned_to_user_id IS NULL`,
			`UPDATE tasks SET last_status_changed_by_user_id = last_status_changed_by WHERE last_status_changed_by IS NOT NULL AND last_status_changed_by_user_id IS NULL`,
			`UPDATE tasks SET completed_at = updated_at WHERE status = 'done' AND completed_at IS NULL`,
		}

		for _, migration := range dataMigrations {
			if err := tx.Exec(migration).Error; err != nil {
				return fmt.Errorf("failed to migrate data: %w", err)
			}
		}

		fmt.Println("✓ Data migrated successfully")
		fmt.Println("🔒 Setting NOT NULL constraint...")

		// Step 3: Set NOT NULL constraint on required columns
		if err := tx.Exec(`ALTER TABLE tasks ALTER COLUMN created_by_user_id SET NOT NULL`).Error; err != nil {
			return fmt.Errorf("failed to set NOT NULL constraint: %w", err)
		}

		fmt.Println("✓ NOT NULL constraint applied")

		// Step 4: Update status constraint to include 'viewed'
		// Drop constraint if exists
		tx.Exec(`ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_status`)

		// Add new constraint
		if err := tx.Exec(`ALTER TABLE tasks ADD CONSTRAINT chk_tasks_status CHECK (status IN ('new', 'viewed', 'in_progress', 'review', 'done', 'cancelled'))`).Error; err != nil {
			// Check if constraint already exists
			if !containsStr(err.Error(), "already exists") {
				fmt.Printf("Warning: failed to add status constraint: %v\n", err)
			}
		}

		// Add progress constraint if not exists
		tx.Exec(`ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_progress_percentage`)
		if err := tx.Exec(`ALTER TABLE tasks ADD CONSTRAINT chk_tasks_progress_percentage CHECK (progress_percentage >= 0 AND progress_percentage <= 100)`).Error; err != nil {
			if !containsStr(err.Error(), "already exists") {
				fmt.Printf("Warning: failed to add progress constraint: %v\n", err)
			}
		}

		// Step 5: Create indexes for new columns
		indexes := []string{
			`CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id ON tasks(parent_task_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tasks_created_by_user_id ON tasks(created_by_user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to_user_id ON tasks(assigned_to_user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tasks_delegated_from_user_id ON tasks(delegated_from_user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to_department_id ON tasks(assigned_to_department_id)`,
			`CREATE INDEX IF NOT EXISTS idx_tasks_last_status_changed_by_user_id ON tasks(last_status_changed_by_user_id)`,
		}

		for _, index := range indexes {
			if err := tx.Exec(index).Error; err != nil {
				// Log but don't fail if index already exists
				fmt.Printf("Warning: %v\n", err)
			}
		}

		// Step 6: Initialize progress for existing tasks
		progressUpdates := []string{
			`UPDATE tasks SET progress_percentage = 100 WHERE status = 'done' AND progress_percentage = 0 AND parent_task_id IS NULL`,
			`UPDATE tasks SET progress_percentage = 50 WHERE status IN ('in_progress', 'review') AND progress_percentage = 0 AND parent_task_id IS NULL`,
		}

		for _, update := range progressUpdates {
			if err := tx.Exec(update).Error; err != nil {
				// Log but don't fail
				fmt.Printf("Warning: %v\n", err)
			}
		}

		// Step 7: Update task_assignees table
		assigneeUpdates := []string{
			`ALTER TABLE task_assignees ADD COLUMN IF NOT EXISTS assigned_by_user_id INTEGER`,
			`ALTER TABLE task_assignees ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`,
		}

		for _, update := range assigneeUpdates {
			if err := tx.Exec(update).Error; err != nil {
				// Log but don't fail
				fmt.Printf("Warning: %v\n", err)
			}
		}

		// Step 8: Recalculate progress for all parent tasks
		// This ensures that parent tasks have correct progress based on their subtasks
		fmt.Println("📊 Recalculating progress for parent tasks...")
		progressRecalculation := `
			WITH subtask_progress AS (
				SELECT
					parent_task_id,
					AVG(progress_percentage)::int as avg_progress
				FROM tasks
				WHERE parent_task_id IS NOT NULL
				  AND deleted_at IS NULL
				GROUP BY parent_task_id
			)
			UPDATE tasks
			SET
				progress_percentage = sp.avg_progress,
				updated_at = NOW()
			FROM subtask_progress sp
			WHERE tasks.id = sp.parent_task_id
			  AND (tasks.progress_percentage != sp.avg_progress OR tasks.progress_percentage IS NULL)
		`
		if err := tx.Exec(progressRecalculation).Error; err != nil {
			// Log but don't fail - this is not critical
			fmt.Printf("Warning: failed to recalculate parent task progress: %v\n", err)
		} else {
			var updatedCount int64
			tx.Raw("SELECT COUNT(*) FROM tasks WHERE id IN (SELECT DISTINCT parent_task_id FROM tasks WHERE parent_task_id IS NOT NULL AND deleted_at IS NULL)").Scan(&updatedCount)
			fmt.Printf("✓ Recalculated progress for parent tasks (checked %d tasks)\n", updatedCount)
		}

		return nil
	})
}
