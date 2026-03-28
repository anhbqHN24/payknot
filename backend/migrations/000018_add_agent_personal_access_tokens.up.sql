CREATE TABLE IF NOT EXISTS agent_personal_access_tokens (
  id UUID PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_name TEXT NOT NULL,
  token_prefix TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  scope TEXT NOT NULL DEFAULT 'agent:runtime',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS agent_personal_access_tokens_user_idx
  ON agent_personal_access_tokens(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS agent_personal_access_tokens_active_idx
  ON agent_personal_access_tokens(revoked_at, expires_at);
