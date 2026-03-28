# Payknot Agent Heartbeat

Use this as the runtime operating cadence for autonomous Payknot agents.

## Every cycle

1. Verify identity is still valid:
   - `GET /api/auth/me` when using PAT for host APIs
   - `GET /api/agent/auth/me` when using agent JWT
2. Confirm the target workspace or host account has not changed unexpectedly.
3. Inspect whether the requested action is read-only or write-impacting.
4. Only then proceed with event/payment work.

## Safe retry policy

- Retry `GET` requests on transient `429` or `5xx` with backoff.
- Do not blindly retry `POST`, `PUT`, or `DELETE`.
- For payment or settlement operations, require idempotency or explicit human confirmation before retrying.

## Drift detection

Stop and escalate if any of these change unexpectedly:
- host account email
- event merchant wallet
- event deposit amount
- checkout expiration timestamp
- PAT revoked or expired

## Mandatory escalation triggers

- `401 Unauthorized` after previously valid PAT/JWT
- repeated `429` from payment-detection endpoints
- mismatch between expected event metadata and current API response
- any paid transaction that cannot be matched back to the expected event and participant
- missing or invalid request signature on a payment-affecting automation call

## Runtime report format

Every significant run should produce:

- auth mode used
- authenticated owner or agent id
- endpoint touched
- write operations attempted
- payment references or tx signatures created
- failures that require human review

## Local memory expectations

Keep only:
- base URL
- owner email
- PAT token hint or token id
- active agent id
- last verified timestamp

Never store:
- full PAT value in ordinary memory files
- JWTs longer than needed
- private keys in non-secret files
