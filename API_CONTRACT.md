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

## Agent Runtime Auth + Settlement Automation
- `GET /agent/auth/nonce?agent_pubkey=<base58>`
  - returns nonce challenge + expiry
- `POST /agent/auth/token`
  - body: `{ agent_pubkey, nonce, signature }`
  - returns `{ access_token, token_type, expires_in }` (JWT)
- `POST /agent/checkout/create`
  - auth: `Authorization: Bearer <agent_jwt>`
  - body: `{ event_id, recipient, amount_usdc, memo }`
  - returns `{ tx_signature, explorer_url }` on success

## Headless v1 Payment Sessions (auth or agent signature)
- `POST /v1/payment-sessions`
  - body: `{ eventId, paymentMethod, walletAddress?, participantData }`
  - creates server-owned payment session + reference
- `GET /v1/payment-sessions/{sessionId}/status`
  - returns session state
- `POST /v1/payment-sessions/{sessionId}/wallet-instructions`
  - returns transfer instructions (`reference`, `merchantWallet`, `amountAtomic`, `mint`)
- `POST /v1/payment-sessions/{sessionId}/submit-signature`
  - body: `{ signature }`
  - verifies and finalizes payment
- `POST /v1/payment-sessions/{sessionId}/verify`
  - verifies existing submitted signature/status
- `POST /v1/payment-sessions/{sessionId}/cancel`
  - cancels unpaid session

## Participant Checkout (legacy public)
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
