# Payknot Agent Skill

Use this skill when acting as an autonomous agent for a Payknot host account.

## Identity bootstrap

Preferred runtime auth:
1. Ask the host to create a Personal Access Token (PAT) from Payknot.
2. Store the PAT in runtime secret storage only. Do not write it into normal logs.
3. For host-management APIs, send `Authorization: Bearer <PAT>`.
4. For settlement automation:
   - generate a local ephemeral Ed25519 keypair
   - call `POST /api/agent/auth/pat` with `{ "token": "<PAT>", "session_pubkey": "<base58_pubkey>", "label": "runtime-name" }`
   - use the returned bearer JWT
   - sign each sensitive request with `X-Agent-Timestamp` and `X-Agent-Signature`

Optional advanced auth:
- Use Ed25519 request-signature mode for host APIs if the host explicitly wants per-agent key management.
- Use nonce -> JWT flow if the runtime already manages an Ed25519 keypair.

## Required discovery order

1. `/llms.txt`
2. `/openapi.json`
3. `/.well-known/agent.json`
4. `/.well-known/agent-integration.json`
5. `/agent-docs/INDEX.md`

## Core host APIs

- `GET /api/auth/me`
- `GET /api/events`
- `POST /api/events`
- `PUT /api/events/{id}`
- `DELETE /api/events/{id}`
- `POST /api/events/import/luma`
- `GET /api/checkout/{slug}`
- `GET /api/checkout/status`
- `GET /api/checkout/participant-status`

## Agent-runtime APIs

- `POST /api/agent/auth/pat`
- `GET /api/agent/auth/me`
- `POST /api/agent/auth/nonce`
- `POST /api/agent/auth/token`
- `POST /api/agent/checkout/create`

## PAT management APIs

These require a normal user session and should only be used by the host:

- `GET /api/agent/pats`
- `POST /api/agent/pats`
- `POST /api/agent/pats/revoke`

## Operating rules

- Never create or revoke PATs from an unattended runtime unless the host explicitly approved it.
- Treat PATs as root-level account credentials for API access.
- Do not use PAT alone for payment-impacting automation.
- Re-auth on `401`, but do not blindly retry the same write request without checking idempotency.
- Before creating a new event, inspect existing events to avoid duplication.
- Before submitting an automated checkout transaction, confirm amount, recipient, and memo are exactly intended.
- Persist only non-sensitive runtime context locally:
  - base URL
  - owner email
  - token id or token hint
  - last successful auth check

## Minimum startup check

1. `GET /api/auth/me` with PAT bearer auth.
2. `GET /api/events` with PAT bearer auth.
3. If payment automation is needed, bootstrap a signed session with `POST /api/agent/auth/pat`.
4. `GET /api/agent/auth/me` with returned JWT.

If any step fails, stop and surface the exact status code and endpoint.
