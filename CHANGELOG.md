# Changelog

## Unreleased
- Reworked event create/edit flow into three sections: Event Info, Checkout Form, Participant Deposits.
- Added event source selection (`custom`, `luma`) and backend Luma public page import API.
- Replaced invite-code checkout with dynamic participant form schema (default name/email + custom fields).
- Added per-event payment method config (`wallet`, `qr`) and AntD Steps-based checkout UX.
- Removed host approve/reject moderation actions from participant deposits.
- Added migration `000010_checkout_form_and_source` for event source/form/methods and participant data snapshot.
- Hardened manual verification by enforcing amount and sender checks for direct transfer verification.
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
