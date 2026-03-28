# API Contract

Base: `/api`

## Auth
- `POST /auth/register`
  - body: `{ name, email, password }`
  - returns: `{ user }`, sets session cookie
- `POST /auth/login`
  - body: `{ email, password }`
  - returns: `{ user }`, sets session cookie
- `POST /auth/google`
  - body: `{ credential }`
  - returns: `{ user }`, sets session cookie
- `GET /auth/me`
  - auth required
  - returns: `{ user }`
- `POST /auth/logout`
  - revokes current session, clears cookie

## Host Event Management (auth required, supports agent signature auth)

Host auth can now be satisfied by:
- normal session cookie
- Personal Access Token (PAT) via `Authorization: Bearer <PAT>`
- Ed25519 agent signature mode
- `GET /events`
  - returns current user-owned events (or `owner_email=agent:<agent-id>` for agent signatures)
- `POST /events`
  - body: event fields
  - creates event with source metadata + participant form schema + payment methods
- `PUT /events/{id}`
  - body: event fields
  - updates event if owned by current user/agent owner identity
- `DELETE /events/{id}`
  - deletes event if owned by current user/agent owner identity
- `POST /events/import/luma`
  - body: `{ url }`
  - imports metadata from public Luma event pages
- `GET /events/{id}/checkouts`
  - returns participant deposit rows

## Agent Key Registry (host session auth required)
- `GET /agent-keys`
  - lists configured agent keys (active/revoked)
- `POST /agent-keys`
  - body: `{ agentId, publicKeyBase64 }`
  - creates or updates (upsert) an agent key
- `POST /agent-keys/revoke`
  - body: `{ agentId }`
  - revokes an agent key

## Personal Access Tokens (host session required)

- `GET /agent/pats`
  - lists PAT metadata for current host account
- `POST /agent/pats`
  - body: `{ name, expiresInDays? }`
  - creates PAT and returns secret token exactly once
- `POST /agent/pats/revoke`
  - body: `{ tokenId }`
  - revokes PAT

## Agent Runtime Auth

- `POST /agent/auth/pat`
  - body: `{ token, session_pubkey?, label? }`
  - exchanges PAT for short-lived runtime JWT
  - when `session_pubkey` is provided, the JWT is bound to that Ed25519 public key for signed payment automation
- `GET /agent/auth/me`
  - returns current runtime identity for PAT/JWT auth

## Signed payment automation

- `POST /agent/checkout/create`
  - requires bearer JWT plus:
    - `X-Agent-Timestamp`
    - `X-Agent-Signature`
  - request must be signed by the Ed25519 private key matching the JWT-bound public key

## Participant Checkout (public)
- `GET /checkout/{slug}`
  - returns event checkout metadata
- `POST /checkout/invoice`
  - body: `{ slug, walletAddress, participantData, paymentMethod }`
  - creates pending checkout reference
- `POST /checkout/confirm`
  - body: `{ reference, signature }`
  - verifies tx and marks paid
- `POST /checkout/recheck`
  - body: `{ reference, signature? }`
  - retries verification/reconciliation
- `POST /checkout/manual-verify`
  - body: `{ slug, walletAddress, signature, participantData? }`
  - verifies manual transfer directly
- `POST /checkout/cancel`
  - body: `{ reference }`
  - removes abandoned pending records
- `GET /checkout/status?reference=...`
  - returns receipt state

## Agent Signature Auth Headers (for supported host endpoints)
- `X-Agent-Id: <agent-id>`
- `X-Agent-Timestamp: <unix-seconds>`
- `X-Agent-Signature: <base64-ed25519-signature>`

Canonical signed payload:
`METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256_HEX(BODY_RAW)`

## Error Shape
- Current implementation returns plain text errors with HTTP status codes.
- Recommendation: migrate to `{ code, message, details? }` for consistency.
