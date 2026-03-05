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

## Host Event Management (auth required)
- `GET /events`
  - returns current user-owned events
- `POST /events`
  - body: event fields
  - creates event + seed invite codes
- `PUT /events/{id}`
  - body: event fields
  - updates event if owned by current user
- `DELETE /events/{id}`
  - deletes event if owned by current user
- `GET /events/{id}/invite-codes`
  - returns invite code list
- `POST /events/{id}/invite-codes`
  - body: `{ count, length? }`
  - appends new invite codes
- `GET /events/{id}/checkouts`
  - returns participant deposit rows

## Host Deposit Moderation (auth required)
- `POST /checkouts/{id}/approve`
  - body: `{ approvedBy, notes? }`
- `POST /checkouts/{id}/reject`
  - body: `{ rejectedBy, reason }`

## Participant Checkout (public)
- `GET /checkout/{slug}`
  - returns event checkout metadata
- `POST /invite/validate`
  - body: `{ slug, code }`
- `POST /invite/status`
  - body: `{ slug, code }`
  - returns validity + existing receipt if present
- `POST /checkout/invoice`
  - body: `{ slug, inviteCode, walletAddress }`
  - creates pending checkout reference
- `POST /checkout/confirm`
  - body: `{ reference, signature }`
  - verifies tx and marks paid
- `POST /checkout/recheck`
  - body: `{ reference, signature? }`
  - retries verification/reconciliation
- `POST /checkout/manual-verify`
  - body: `{ slug, inviteCode, walletAddress, signature }`
  - verifies manual transfer directly
- `POST /checkout/cancel`
  - body: `{ reference }`
  - removes abandoned pending records
- `GET /checkout/status?reference=...`
  - returns receipt state

## Error Shape
- Current implementation returns plain text errors with HTTP status codes.
- Recommendation: migrate to `{ code, message, details? }` for consistency.
