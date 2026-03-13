CREATE TABLE IF NOT EXISTS agent_nonces (
  id BIGSERIAL PRIMARY KEY,
  agent_pubkey TEXT NOT NULL,
  nonce TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS agent_nonces_pubkey_idx
  ON agent_nonces(agent_pubkey, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_sessions (
  id UUID PRIMARY KEY,
  agent_pubkey TEXT NOT NULL,
  jwt_jti TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS agent_sessions_pubkey_idx
  ON agent_sessions(agent_pubkey, created_at DESC);

CREATE TABLE IF NOT EXISTS automation_policies (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  max_amount_usdc NUMERIC(20,6) NOT NULL DEFAULT 500,
  hourly_tx_limit INT NOT NULL DEFAULT 10,
  daily_amount_limit_usdc NUMERIC(20,6),
  recipient_allowlist JSONB,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS automation_intents (
  id UUID PRIMARY KEY,
  agent_pubkey TEXT NOT NULL,
  event_id BIGINT,
  recipient_wallet TEXT NOT NULL,
  amount_usdc NUMERIC(20,6) NOT NULL,
  memo TEXT NOT NULL,
  status TEXT NOT NULL,
  tx_signature TEXT,
  policy_reason TEXT,
  idempotency_key TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS automation_intents_agent_idx
  ON automation_intents(agent_pubkey, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS automation_intents_agent_idem_idx
  ON automation_intents(agent_pubkey, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

INSERT INTO automation_policies (name, max_amount_usdc, hourly_tx_limit, active)
VALUES ('default', 500, 10, TRUE)
ON CONFLICT (name) DO NOTHING;
