-- Create password_resets table for secure password reset flow
-- Tokens are one-time use and expire after 24 hours

CREATE TABLE IF NOT EXISTS password_resets (
    id SERIAL PRIMARY KEY,
    token VARCHAR(255) NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_by_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_password_resets_token ON password_resets(token);
CREATE INDEX idx_password_resets_user_id ON password_resets(user_id);
CREATE INDEX idx_password_resets_email ON password_resets(email);
CREATE INDEX idx_password_resets_status ON password_resets(status);
CREATE INDEX idx_password_resets_expires_at ON password_resets(expires_at);

-- Add comment
COMMENT ON TABLE password_resets IS 'Password reset tokens for secure password reset flow';
COMMENT ON COLUMN password_resets.status IS 'pending, used, expired, or cancelled';
