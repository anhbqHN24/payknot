DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS payment_attempts;
DROP INDEX IF EXISTS payment_sessions_owner_idem_idx;
DROP INDEX IF EXISTS payment_sessions_owner_idx;
DROP INDEX IF EXISTS payment_sessions_event_state_idx;
DROP TABLE IF EXISTS payment_sessions;
