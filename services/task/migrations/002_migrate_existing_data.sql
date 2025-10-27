-- Migration script to update existing tasks table structure
-- This should be run BEFORE starting the updated task service

-- Step 1: Add new columns as nullable first (to allow data migration)
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS created_by_user_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_to_user_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS delegated_from_user_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS original_assignee_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_to_department_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS parent_task_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS progress_percentage INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS first_viewed_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS first_viewed_by_user_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS last_status_changed_by_user_id INTEGER;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP;

-- Step 2: Migrate data from old columns to new columns
UPDATE tasks
SET created_by_user_id = created_by
WHERE created_by IS NOT NULL AND created_by_user_id IS NULL;

UPDATE tasks
SET assigned_to_user_id = assigned_to
WHERE assigned_to IS NOT NULL AND assigned_to_user_id IS NULL;

UPDATE tasks
SET last_status_changed_by_user_id = last_status_changed_by
WHERE last_status_changed_by IS NOT NULL AND last_status_changed_by_user_id IS NULL;

-- Step 3: Set NOT NULL constraint on required columns after data migration
ALTER TABLE tasks ALTER COLUMN created_by_user_id SET NOT NULL;

-- Step 4: Add progress percentage constraint
ALTER TABLE tasks ADD CONSTRAINT chk_tasks_progress_percentage
    CHECK (progress_percentage >= 0 AND progress_percentage <= 100);

-- Step 5: Create indexes for new columns
CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id ON tasks(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_by_user_id ON tasks(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to_user_id ON tasks(assigned_to_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_delegated_from_user_id ON tasks(delegated_from_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to_department_id ON tasks(assigned_to_department_id);
CREATE INDEX IF NOT EXISTS idx_tasks_last_status_changed_by_user_id ON tasks(last_status_changed_by_user_id);

-- Step 6: Add foreign key for parent_task_id (self-reference)
ALTER TABLE tasks ADD CONSTRAINT fk_tasks_parent
    FOREIGN KEY (parent_task_id) REFERENCES tasks(id) ON DELETE CASCADE;

-- Step 7: Update status constraint to include 'viewed'
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_status;
ALTER TABLE tasks ADD CONSTRAINT chk_tasks_status
    CHECK (status IN ('new', 'viewed', 'in_progress', 'review', 'done', 'cancelled'));

-- Step 8: Update task_assignees table to add new columns
ALTER TABLE task_assignees ADD COLUMN IF NOT EXISTS assigned_by_user_id INTEGER;
ALTER TABLE task_assignees ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Step 9: Set completed_at for already completed tasks
UPDATE tasks
SET completed_at = updated_at
WHERE status = 'done' AND completed_at IS NULL;

-- Step 10: Initialize progress for existing tasks
-- Tasks without subtasks and status 'done' get 100%
UPDATE tasks
SET progress_percentage = 100
WHERE status = 'done'
  AND progress_percentage = 0
  AND parent_task_id IS NULL
  AND id NOT IN (SELECT DISTINCT parent_task_id FROM tasks WHERE parent_task_id IS NOT NULL);

-- Tasks in progress without subtasks get 50%
UPDATE tasks
SET progress_percentage = 50
WHERE status IN ('in_progress', 'review')
  AND progress_percentage = 0
  AND parent_task_id IS NULL
  AND id NOT IN (SELECT DISTINCT parent_task_id FROM tasks WHERE parent_task_id IS NOT NULL);

-- Note: Keep old columns for backward compatibility
-- They will be populated automatically via GORM hooks
-- Consider removing them in a future migration after all clients are updated

COMMENT ON COLUMN tasks.created_by IS 'Deprecated: Use created_by_user_id instead';
COMMENT ON COLUMN tasks.assigned_to IS 'Deprecated: Use assigned_to_user_id instead';
COMMENT ON COLUMN tasks.last_status_changed_by IS 'Deprecated: Use last_status_changed_by_user_id instead';
COMMENT ON COLUMN tasks.assigned_to_department IS 'Deprecated: Use assigned_to_department_id instead';
