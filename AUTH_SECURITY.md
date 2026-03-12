# Auth & Security

## Session Model
- JWT is signed server-side (HS256) and stored in HttpOnly cookie.
- Backend validates both:
  1. JWT signature/expiry
  2. active row in `user_sessions`

## Cookie
- Name configurable via `AUTH_COOKIE_NAME` (default `spw_session`)
- `HttpOnly`, `SameSite=Lax`
- `Secure` controlled by `AUTH_COOKIE_SECURE` (or TLS detection)

## Providers
- Password auth (bcrypt hashes)
- Google auth (`/auth/google`) with backend token verification

## Required Env
- `AUTH_JWT_SECRET` (must be long/random)
- `GOOGLE_CLIENT_ID` (if Google auth enabled)

## Access Control
- `/events*` and `/checkouts*` host moderation endpoints require authenticated session.
- Ownership enforcement by `owner_email` in SQL predicates.

## Agent Key Access (Server-to-Server)

For external AI agents/coding assistants (non-browser automation), selected protected routes can also accept an agent key when configured.

### Required Env
- `AGENT_ACCESS_KEY_SHA256`: SHA256 hex hash of the raw agent key
- `AGENT_ACCESS_EMAIL`: owner email identity used for event ownership context

### Request Headers
- `X-Agent-Key: <raw-agent-key>`
  or
- `Authorization: Bearer <raw-agent-key>`

### Security Guidance
- Never expose the raw key in frontend/browser code.
- Rotate the key periodically.
- Store key only in secret manager/runtime env.

## Security Baseline Checklist
- Rotate `AUTH_JWT_SECRET` on incident
- Enable HTTPS and `AUTH_COOKIE_SECURE=true` in production
- Add CSRF protection for cookie-based auth (recommended next)
- Add structured audit logs for approve/reject and auth events
- Add brute-force controls for `/auth/login`
