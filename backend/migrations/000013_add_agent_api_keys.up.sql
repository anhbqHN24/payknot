CREATE TABLE IF NOT EXISTS agent_api_keys (
  id BIGSERIAL PRIMARY KEY,
  agent_id VARCHAR NOT NULL UNIQUE,
  public_key_base64 TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by VARCHAR NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS agent_api_keys_active_idx
  ON agent_api_keys(active, revoked_at);
