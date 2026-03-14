ALTER TABLE users
  ADD COLUMN IF NOT EXISTS password_reset_token_hash VARCHAR,
  ADD COLUMN IF NOT EXISTS password_reset_expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS password_reset_sent_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS users_password_reset_token_idx
  ON users(password_reset_token_hash);
