# Changelog

## Unreleased
- Added backend auth with JWT + HttpOnly cookie sessions and DB-backed session verification.
- Added Google auth backend verification endpoint.
- Protected host event/checkouts APIs with auth middleware.
- Added `users` and `user_sessions` schema migrations.
- Added event management detail UX: desktop side-drawer + mobile full-screen modal.
- Added searchable/paginated event sessions list.
- Added searchable/paginated invite and deposit sublists.
- Added panel open/close animations for mobile and desktop.
- Added reject-with-reason moderation flow surfaced to participant receipt.
- Removed unused legacy frontend component barrel and unused Node watcher helper files.

## Migration Impact
- Requires running migrations through `000009`.
- Requires `AUTH_JWT_SECRET` backend env before auth endpoints can work.
