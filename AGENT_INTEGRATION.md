# AGENT_INTEGRATION.md

Guide for external AI agents / coding assistants to integrate Payknot-style event checkout payment features into another service.

## What this gives you

- Event creation + checkout link generation
- Participant checkout and payment verification
- Host-side deposit review flow
- API-level integration path (without browser login) via agent key

## Recommended reading order

1. `SKILL.md`
2. `PRODUCT_SPEC.md`
3. `API_CONTRACT.md`
4. `DATABASE_SCHEMA.md`
5. `AUTH_SECURITY.md`
6. `PAYMENT_INTEGRITY.md`
7. `OPERATIONS_RUNBOOK.md`

## API auth modes

### A) User session (default)
- Login/register/google auth
- HttpOnly cookie session

### B) Agent key (for server-to-server / autonomous agents)
Use either header:
- `X-Agent-Key: <plain-agent-key>`
- `Authorization: Bearer <plain-agent-key>`

Server validates SHA256 hash of provided key against env `AGENT_ACCESS_KEY_SHA256`.

### Generate key hash

```bash
AGENT_ACCESS_KEY="replace-with-long-random-secret"
echo -n "$AGENT_ACCESS_KEY" | sha256sum | awk '{print $1}'
```

Set output to backend env:
- `AGENT_ACCESS_KEY_SHA256=<hex-hash>`
- `AGENT_ACCESS_EMAIL=<owner-email-for-created-events>`

## Minimal integration flow for other products

1. Call `POST /api/events` to create event.
2. Use returned checkout slug/url in your product.
3. Call checkout APIs:
   - create invoice
   - confirm/recheck status
   - participant status lookup
4. Consume host event/deposit endpoints for operations.

## Security notes

- Treat agent key as secret; never embed in browser JS.
- Rotate key periodically.
- Restrict key use to backend services/agent runners.
- If compromised, regenerate hash and redeploy.
