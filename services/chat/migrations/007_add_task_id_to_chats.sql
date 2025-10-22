-- Add task_id column to chats table for linking chats to tasks
-- Migration: 007_add_task_id_to_chats

-- Add task_id column (nullable foreign key to task-service)
ALTER TABLE chats ADD COLUMN IF NOT EXISTS task_id INTEGER;

-- Add index for task_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_chats_task_id ON chats(task_id);

-- Add comment to explain the column
COMMENT ON COLUMN chats.task_id IS 'Links chat to a task (nullable, for task-related group chats)';
