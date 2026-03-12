---
name: event-deposit-payment-integration
description: Implement an event deposit checkout/payment system in another product using this repository as a reference baseline. Use when building hosted checkout sessions, participant payment verification, invite-code validation, host approval/rejection, wallet/QR payment modes, and production deployment/operations for a Solana USDC event-deposit flow.
---

# Event Deposit Payment Integration Skill

## 1) Read in this order

1. `PRODUCT_SPEC.md` (business behavior)
2. `API_CONTRACT.md` (endpoints + payload contracts)
3. `DATABASE_SCHEMA.md` (tables, statuses, migrations)
4. `AUTH_SECURITY.md` (session/auth/cookie model)
5. `PAYMENT_INTEGRITY.md` (verification + anti-fraud rules)
6. `OPERATIONS_RUNBOOK.md` (runtime operations)
7. `QA_UAT_TEST_PLAN.md` (acceptance checklist)

Use `PRIVACY_POLICY.md` and `TERMS_OF_SERVICE.md` when preparing external release/legal pages.

## 2) Implement minimal vertical slice

1. Create checkout session API (event + participant info + payment method).
2. Generate payment reference and enforce state machine (`pending_payment` -> `paid` -> approval states).
3. Verify on-chain payment (signature/reference/mint/amount consistency).
4. Persist receipt and expose participant status lookup endpoint.
5. Build participant checkout UI:
   - participant form
   - payment method selection (wallet/QR)
   - pay + receipt status
6. Build host controls for approve/reject with reason.

## 3) Non-negotiable invariants

- Never mark paid before verification succeeds.
- Bind each payment to event + participant + expected amount.
- Store immutable transaction reference/signature for auditing.
- Keep auth/session handling server-side (avoid exposing secrets in client).
- Treat migration order as release-critical.

## 4) Deployment requirements

- Apply DB migrations before application rollout.
- Verify API unauth expectations (`/api/auth/me` and protected endpoints return 401 when unauthenticated).
- Run smoke checks after deploy (homepage + checkout route + payment status path).

## 5) Adaptation notes for other products

- Keep the same domain model and status transitions even if UI/stack differs.
- If blockchain/network changes, preserve verification semantics (asset, amount, receiver, reference).
- If auth provider changes, keep session revocation and secure-cookie principles.
