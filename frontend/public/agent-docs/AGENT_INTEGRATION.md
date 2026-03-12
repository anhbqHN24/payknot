# AGENT_INTEGRATION.md

Guide for external AI agents / coding assistants to integrate Payknot-style event checkout payment features into another service.

## What this gives you

- Event creation + checkout link generation
- Participant checkout and payment verification
- Host-side deposit review flow
- API-level integration path (without browser login) via agent signatures
- Per-agent ownership attribution via `owner_email = agent:<agent-id>`

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

### B) Agent signature auth (server-to-server)
Headers required:
- `X-Agent-Id: <agent-id>`
- `X-Agent-Timestamp: <unix-seconds>`
- `X-Agent-Signature: <base64-ed25519-signature>`

Signed message format:

```text
<METHOD>\n<PATH>\n<TIMESTAMP>\n<SHA256_HEX_OF_RAW_BODY>
```

Example canonical message for `POST /api/events`:

```text
POST
/api/events
1710000000
<sha256-hex-of-json-body>
```

### Server key registry
Preferred (runtime change without redeploy):
- store keys in DB table `agent_api_keys`
- manage via host APIs:
  - `GET /api/agent-keys`
  - `POST /api/agent-keys` (upsert)
  - `POST /api/agent-keys/revoke`

Fallback/bootstrap:
- `AGENT_PUBLIC_KEYS_JSON` (JSON object)
- Format: `{"agent-a":"<base64-public-key>","agent-b":"<base64-public-key>"}`

## Minimal integration flow for other products

1. Sign request and call `POST /api/events`.
2. Use returned checkout slug/url in your product.
3. Call checkout APIs:
   - create invoice
   - confirm/recheck status
   - participant status lookup
4. Consume host event/deposit endpoints for operations.

## Security notes

- Never embed private signing keys in frontend/browser code.
- Keep private keys in agent runtime secret stores.
- Rotate per-agent keypairs and update server public key map.
- Timestamps are validated in a short time window to reduce replay risk.
