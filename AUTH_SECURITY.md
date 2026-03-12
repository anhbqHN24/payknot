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

## Agent Signature Access (Server-to-Server)

For external AI agents/coding assistants (non-browser automation), selected protected routes can also accept signed requests.

### Required Env
- `AGENT_PUBLIC_KEYS_JSON`: JSON object mapping `agent_id -> base64(ed25519 public key)`

### Required Headers
- `X-Agent-Id: <agent-id>`
- `X-Agent-Timestamp: <unix-seconds>`
- `X-Agent-Signature: <base64-ed25519-signature>`

### Signature Canonical String

`METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256_HEX(BODY_RAW)`

### Ownership Attribution
- Valid signed requests run under synthetic identity: `owner_email = "agent:<agent-id>"`
- This leaves per-agent ownership marks for created/managed events.

### Security Guidance
- Never expose private keys in frontend/browser code.
- Rotate keypairs periodically and update `AGENT_PUBLIC_KEYS_JSON`.
- Keep timestamps in short windows and use HTTPS.

## Security Baseline Checklist
- Rotate `AUTH_JWT_SECRET` on incident
- Enable HTTPS and `AUTH_COOKIE_SECURE=true` in production
- Add CSRF protection for cookie-based auth (recommended next)
- Add structured audit logs for approve/reject and auth events
- Add brute-force controls for `/auth/login`
