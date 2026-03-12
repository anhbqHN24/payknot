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
  - creates event with source metadata + participant form schema + payment methods
- `PUT /events/{id}`
  - body: event fields
  - updates event if owned by current user
- `DELETE /events/{id}`
  - deletes event if owned by current user
- `POST /events/import/luma`
  - body: `{ url }`
  - imports metadata from public Luma event pages
- `GET /events/{id}/checkouts`
  - returns participant deposit rows

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

## Error Shape
- Current implementation returns plain text errors with HTTP status codes.
- Recommendation: migrate to `{ code, message, details? }` for consistency.
