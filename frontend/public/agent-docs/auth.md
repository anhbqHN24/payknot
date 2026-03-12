# Agent Auth Guide

## Signature headers

Required headers for agent-signed host endpoints:

- `X-Agent-Id`
- `X-Agent-Timestamp` (unix seconds)
- `X-Agent-Signature` (base64 Ed25519 signature)

## Canonical signing string

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

## Verification model

- Server resolves `agent_id` to active public key in `agent_api_keys` table.
- Server verifies signature and timestamp window.
- Ownership context is attributed to `owner_email = agent:<agent-id>`.

## Failure modes

- Invalid signature -> `401 Unauthorized`
- Unknown/revoked `agent_id` -> `401 Unauthorized`
- Timestamp outside allowed window -> `401 Unauthorized`

## Security notes

- Keep private keys server-side only.
- Rotate keypairs periodically.
- Revoke old keys immediately if compromised.
