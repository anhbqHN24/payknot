DROP INDEX IF EXISTS users_email_verification_token_idx;

ALTER TABLE users
  DROP COLUMN IF EXISTS email_verification_sent_at,
  DROP COLUMN IF EXISTS email_verification_expires_at,
  DROP COLUMN IF EXISTS email_verification_token_hash;
