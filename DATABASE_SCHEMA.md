# Database Schema & Migrations

## Core Existing
- `invoice`
- `paid_wallet` (kept as view per project decision)

## Event Checkout Domain
- `events`
  - owner_email-scoped
  - metadata + merchant wallet + amount + network
- `invite_codes`
  - unique code, usage counters, status, optional expiry
- `event_checkouts`
  - participant wallet, reference, signature, status lifecycle, moderation fields

`checkout_status` enum includes:
- `pending_payment`
- `paid`
- `approved`
- `failed`
- `rejected`

## Auth Domain
- `users`
  - `email` unique
  - provider: `password` or `google`
  - password hash nullable for OAuth users
- `user_sessions`
  - session id, user id, expiry, revoke timestamp, ip/user-agent

## Key Migration Files
- `000005_event_checkout_platform.*`
- `000006_add_event_image.*`
- `000007_add_event_owner_fields.*`
- `000008_add_rejected_checkout_status.*`
- `000009_add_auth_tables.*`

## Migration Notes
- Apply migrations in order.
- `000008` down migration is no-op due PostgreSQL enum value removal limitations.
- Always test migration on staging DB before production.
