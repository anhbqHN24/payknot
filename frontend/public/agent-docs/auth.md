# Agent Auth Guide

## Mode 0: Personal Access Token (recommended for practical agent integrations)

1. Host creates a PAT via `POST /api/agent/pats` from a normal logged-in session.
2. Agent stores the returned token in secret storage.
3. Use `Authorization: Bearer <PAT>` for host APIs such as:
   - `GET /api/auth/me`
   - `GET /api/events`
   - `POST /api/events`
   - `POST /api/events/import/luma`
4. For payment automation, generate an ephemeral Ed25519 keypair locally.
5. Exchange PAT via `POST /api/agent/auth/pat` with:
   - `token`
   - `session_pubkey`
   - optional `label`
6. Use returned JWT for runtime auth and sign sensitive requests with:
   - `X-Agent-Timestamp`
   - `X-Agent-Signature`

## Mode A: Nonce -> JWT (primary for runtime automation)

1. `GET /api/agent/auth/nonce?agent_pubkey=<base58_pubkey>`
2. Sign nonce with agent private key (Ed25519)
3. `POST /api/agent/auth/token` with:
   - `agent_pubkey`
   - `nonce`
   - `signature` (base58)
4. Use `Authorization: Bearer <access_token>` for:
   - `POST /api/agent/checkout/create`

Token TTL: 24h (re-auth on 401).

For payment-impacting runtime writes, JWT alone is not enough. The request must also be signed by the Ed25519 private key that matches the JWT-bound public key.

## Mode B: Request signature auth (supported for signed host APIs)

Required headers:
- `X-Agent-Id`
- `X-Agent-Timestamp` (unix seconds)
- `X-Agent-Signature` (base64 Ed25519 signature)

Canonical signing string:

```text
METHOD
PATH
TIMESTAMP
SHA256_HEX(BODY_RAW)
```

Example:

```text
POST
/api/events
1710000000
<sha256-hex-of-json-body>
```

## Failure modes

- Invalid nonce/signature/token -> `401 Unauthorized`
- Policy rejection on settlement -> `403 Forbidden`
- Timestamp outside allowed window (signature mode) -> `401 Unauthorized`

## Security notes

- Treat PAT as a root API credential for the host account.
- Keep private keys server-side only.
- Never log JWTs or private keys.
- Rotate keypairs periodically.
- Revoke old keys immediately if compromised.
