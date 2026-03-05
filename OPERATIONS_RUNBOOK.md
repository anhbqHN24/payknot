# Operations Runbook

## Services
- Frontend: Next.js (`frontend`)
- Backend: Go API (`backend`)
- PostgreSQL
- Redis

## Environment
Backend minimum:
- `DATABASE_URL`
- `REDIS_URL`
- `SOLANA_RPC_URL`
- `USDC_MINT`
- `MERCHANT_WALLET` (legacy paywall)
- `AUTH_JWT_SECRET`
- optional: `GOOGLE_CLIENT_ID`, `AUTH_COOKIE_*`

Frontend minimum:
- `NEXT_PUBLIC_SOLANA_RPC_URL`
- `NEXT_PUBLIC_USDC_MINT`
- `NEXT_PUBLIC_GOOGLE_CLIENT_ID` (for Google button)

## Startup
1. Start Postgres + Redis
2. Apply migrations
3. Start backend on `:8080`
4. Start frontend on `:3000`

## Health Checks
- `GET /api/auth/me` (expects 401 when unauthenticated)
- `GET /api/events` (expects 401 when unauthenticated)
- Participant checkout page loads event by slug

## Incident Triage
- Auth failures: check cookie secure flag + JWT secret + session rows
- Payment verify failures: check RPC health, mint/address mismatches, signature validity
- Missing records: run recheck/manual verification path

## Backups
- Daily DB backup
- Store migration history with release artifacts
