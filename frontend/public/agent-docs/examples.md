# Agent Integration Examples

## 1) Happy path: create event (signed)

1. Build JSON body for `POST /api/events`.
2. Compute canonical string:

```text
POST
/api/events
<TS>
<SHA256_HEX(body)>
```

3. Sign canonical string with Ed25519 private key.
4. Send request with:
   - `X-Agent-Id`
   - `X-Agent-Timestamp`
   - `X-Agent-Signature`

Expected result: `200` with event id/slug/checkoutUrl.

## 2) Happy path: participant checkout payment

1. `POST /api/checkout/invoice`
2. User pays through wallet or QR route.
3. `POST /api/checkout/confirm`
4. Poll `GET /api/checkout/status?reference=...`

Expected result: status transitions to `paid` and receipt contains reference/signature metadata.

## 3) Failure path: invalid signature

- Send signed request with wrong body hash or wrong timestamp.
- Expected: `401 Unauthorized`

## 4) Failure path: revoked agent key

- Revoke key via host API `POST /api/agent-keys/revoke`.
- Retry same signed request.
- Expected: `401 Unauthorized`

## 5) Failure path: no completed transaction

- Call participant status check for unknown participant.
- Expected: not-found/negative status response (do not assume paid).
