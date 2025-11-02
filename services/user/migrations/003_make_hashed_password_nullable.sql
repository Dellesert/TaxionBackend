-- Make hashed_password nullable for invitation-based registration
-- Users can now be created without passwords and set them when accepting invitations

ALTER TABLE users ALTER COLUMN hashed_password DROP NOT NULL;
