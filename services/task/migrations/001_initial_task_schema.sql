-- Initial migration for task service with hierarchical task system
-- This file is for reference, GORM will handle migrations automatically

-- Create tasks table with hierarchical structure
CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,

    -- Basic information
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'new',
    priority VARCHAR(20) NOT NULL DEFAULT 'medium',

    -- Hierarchy support
    parent_task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,

    -- Assignment and delegation
    created_by_user_id INTEGER NOT NULL,
    assigned_to_user_id INTEGER,
    delegated_from_user_id INTEGER,
    original_assignee_id INTEGER,
    assigned_to_department_id INTEGER,

    -- Progress tracking
    progress_percentage INTEGER DEFAULT 0 CHECK (progress_percentage BETWEEN 0 AND 100),
    first_viewed_at TIMESTAMP NULL,
    first_viewed_by_user_id INTEGER,
    last_status_changed_by_user_id INTEGER,

    -- Dates
    due_date TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for tasks table
CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id ON tasks(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_by_user_id ON tasks(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to_user_id ON tasks(assigned_to_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_delegated_from_user_id ON tasks(delegated_from_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to_department_id ON tasks(assigned_to_department_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_due_date ON tasks(due_date);
CREATE INDEX IF NOT EXISTS idx_tasks_deleted_at ON tasks(deleted_at);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);

-- Add constraints for enum-like fields
ALTER TABLE tasks ADD CONSTRAINT chk_tasks_status
    CHECK (status IN ('new', 'viewed', 'in_progress', 'review', 'done', 'cancelled'));

ALTER TABLE tasks ADD CONSTRAINT chk_tasks_priority
    CHECK (priority IN ('low', 'medium', 'high', 'critical'));

-- Create task_assignees table (many-to-many for subtasks with multiple assignees)
CREATE TABLE IF NOT EXISTS task_assignees (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    assigned_by_user_id INTEGER,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    UNIQUE(task_id, user_id, deleted_at)
);

-- Create indexes for task_assignees
CREATE INDEX IF NOT EXISTS idx_task_assignees_task_id ON task_assignees(task_id);
CREATE INDEX IF NOT EXISTS idx_task_assignees_user_id ON task_assignees(user_id);
CREATE INDEX IF NOT EXISTS idx_task_assignees_deleted_at ON task_assignees(deleted_at);

-- Create task_activities table for activity log
CREATE TABLE IF NOT EXISTS task_activities (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    action_type VARCHAR(50) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    details JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for task_activities
CREATE INDEX IF NOT EXISTS idx_task_activities_task_id ON task_activities(task_id);
CREATE INDEX IF NOT EXISTS idx_task_activities_user_id ON task_activities(user_id);
CREATE INDEX IF NOT EXISTS idx_task_activities_action_type ON task_activities(action_type);
CREATE INDEX IF NOT EXISTS idx_task_activities_created_at ON task_activities(created_at DESC);

-- Add constraint for action types
ALTER TABLE task_activities ADD CONSTRAINT chk_task_activities_action_type
    CHECK (action_type IN (
        'created', 'assigned', 'delegated', 'status_changed',
        'priority_changed', 'file_attached', 'file_deleted',
        'viewed', 'completed', 'commented', 'updated',
        'subtask_created', 'subtask_completed', 'reassigned'
    ));

-- Create task_attachments table
CREATE TABLE IF NOT EXISTS task_attachments (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    uploaded_by_user_id INTEGER NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_type VARCHAR(50),
    file_size BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for task_attachments
CREATE INDEX IF NOT EXISTS idx_task_attachments_task_id ON task_attachments(task_id);
CREATE INDEX IF NOT EXISTS idx_task_attachments_uploaded_by_user_id ON task_attachments(uploaded_by_user_id);
CREATE INDEX IF NOT EXISTS idx_task_attachments_deleted_at ON task_attachments(deleted_at);

-- Create task_checklists table
CREATE TABLE IF NOT EXISTS task_checklists (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for task_checklists
CREATE INDEX IF NOT EXISTS idx_task_checklists_task_id ON task_checklists(task_id);
CREATE INDEX IF NOT EXISTS idx_task_checklists_deleted_at ON task_checklists(deleted_at);
CREATE INDEX IF NOT EXISTS idx_task_checklists_position ON task_checklists(position);

-- Create task_checklist_items table
CREATE TABLE IF NOT EXISTS task_checklist_items (
    id SERIAL PRIMARY KEY,
    checklist_id INTEGER NOT NULL REFERENCES task_checklists(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_by_user_id INTEGER,
    completed_at TIMESTAMP NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for task_checklist_items
CREATE INDEX IF NOT EXISTS idx_task_checklist_items_checklist_id ON task_checklist_items(checklist_id);
CREATE INDEX IF NOT EXISTS idx_task_checklist_items_is_completed ON task_checklist_items(is_completed);
CREATE INDEX IF NOT EXISTS idx_task_checklist_items_deleted_at ON task_checklist_items(deleted_at);
CREATE INDEX IF NOT EXISTS idx_task_checklist_items_position ON task_checklist_items(position);

-- Create task_comments table
CREATE TABLE IF NOT EXISTS task_comments (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL,
    parent_comment_id INTEGER REFERENCES task_comments(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- Create indexes for task_comments
CREATE INDEX IF NOT EXISTS idx_task_comments_task_id ON task_comments(task_id);
CREATE INDEX IF NOT EXISTS idx_task_comments_user_id ON task_comments(user_id);
CREATE INDEX IF NOT EXISTS idx_task_comments_parent_comment_id ON task_comments(parent_comment_id);
CREATE INDEX IF NOT EXISTS idx_task_comments_deleted_at ON task_comments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_task_comments_created_at ON task_comments(created_at DESC);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at on all tables
CREATE TRIGGER update_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_assignees_updated_at BEFORE UPDATE ON task_assignees
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_attachments_updated_at BEFORE UPDATE ON task_attachments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_checklists_updated_at BEFORE UPDATE ON task_checklists
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_checklist_items_updated_at BEFORE UPDATE ON task_checklist_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_comments_updated_at BEFORE UPDATE ON task_comments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create function to automatically calculate task progress based on subtasks
CREATE OR REPLACE FUNCTION calculate_task_progress()
RETURNS TRIGGER AS $$
DECLARE
    parent_id INTEGER;
    total_subtasks INTEGER;
    completed_subtasks INTEGER;
    new_progress INTEGER;
BEGIN
    -- Get parent task ID
    IF TG_OP = 'DELETE' THEN
        parent_id := OLD.parent_task_id;
    ELSE
        parent_id := NEW.parent_task_id;
    END IF;

    -- Only process if task has a parent
    IF parent_id IS NOT NULL THEN
        -- Count total subtasks and completed subtasks
        SELECT COUNT(*), COUNT(CASE WHEN status IN ('done', 'cancelled') THEN 1 END)
        INTO total_subtasks, completed_subtasks
        FROM tasks
        WHERE parent_task_id = parent_id AND deleted_at IS NULL;

        -- Calculate progress percentage
        IF total_subtasks > 0 THEN
            new_progress := ROUND((completed_subtasks::DECIMAL / total_subtasks::DECIMAL) * 100);
        ELSE
            new_progress := 0;
        END IF;

        -- Update parent task progress
        UPDATE tasks
        SET progress_percentage = new_progress,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = parent_id;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update parent task progress when subtask status changes
CREATE TRIGGER update_parent_task_progress_on_insert AFTER INSERT ON tasks
    FOR EACH ROW EXECUTE FUNCTION calculate_task_progress();

CREATE TRIGGER update_parent_task_progress_on_update AFTER UPDATE OF status ON tasks
    FOR EACH ROW EXECUTE FUNCTION calculate_task_progress();

CREATE TRIGGER update_parent_task_progress_on_delete AFTER DELETE ON tasks
    FOR EACH ROW EXECUTE FUNCTION calculate_task_progress();
