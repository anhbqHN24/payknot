ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email_verification_token_hash VARCHAR,
  ADD COLUMN IF NOT EXISTS email_verification_expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS email_verification_sent_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS users_email_verification_token_idx
  ON users(email_verification_token_hash);
