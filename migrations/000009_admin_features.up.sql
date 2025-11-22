-- Add admin features (user banning)

-- Add ban fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_banned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS ban_reason TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMP;

-- Index for banned users
CREATE INDEX IF NOT EXISTS idx_users_is_banned ON users(is_banned);

COMMENT ON COLUMN users.is_banned IS 'Whether user is banned from system';
COMMENT ON COLUMN users.ban_reason IS 'Reason for ban (admin notes)';
COMMENT ON COLUMN users.banned_at IS 'When user was banned';

