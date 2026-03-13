# Agent Auth Guide

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

- Keep private keys server-side only.
- Never log JWTs or private keys.
- Rotate keypairs periodically.
- Revoke old keys immediately if compromised.
