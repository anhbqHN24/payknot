CREATE TABLE IF NOT EXISTS payment_sessions (
  id UUID PRIMARY KEY,
  event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  owner_email VARCHAR NOT NULL,
  participant_data JSONB NOT NULL DEFAULT '{}'::jsonb,
  wallet_address VARCHAR NOT NULL DEFAULT '',
  payment_method VARCHAR NOT NULL,
  state VARCHAR NOT NULL,
  reference UUID NOT NULL UNIQUE,
  amount_atomic BIGINT NOT NULL,
  mint VARCHAR NOT NULL,
  merchant_wallet VARCHAR NOT NULL,
  idempotency_key VARCHAR,
  signature VARCHAR,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS payment_sessions_event_state_idx
  ON payment_sessions(event_id, state);
CREATE INDEX IF NOT EXISTS payment_sessions_owner_idx
  ON payment_sessions(owner_email, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS payment_sessions_owner_idem_idx
  ON payment_sessions(owner_email, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS payment_attempts (
  id BIGSERIAL PRIMARY KEY,
  session_id UUID NOT NULL REFERENCES payment_sessions(id) ON DELETE CASCADE,
  signature VARCHAR NOT NULL,
  status VARCHAR NOT NULL,
  error_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS idempotency_records (
  id BIGSERIAL PRIMARY KEY,
  owner_email VARCHAR NOT NULL,
  idem_key VARCHAR NOT NULL,
  endpoint VARCHAR NOT NULL,
  response_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(owner_email, idem_key, endpoint)
);
