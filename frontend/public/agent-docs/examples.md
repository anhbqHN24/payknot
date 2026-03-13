# Agent Integration Examples

## 1) Happy path: agent runtime auth (nonce -> JWT)

1. `GET /api/agent/auth/nonce?agent_pubkey=<base58_pubkey>`
2. Sign returned nonce with Ed25519 private key
3. `POST /api/agent/auth/token`
4. Store `access_token` and send as `Authorization: Bearer ...`

Expected result: bearer token ready for `/api/agent/checkout/create`.

## 1b) Happy path: create event (signed mode)

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

## 2) Happy path: participant checkout payment (headless v1)

1. `POST /api/v1/payment-sessions`
2. Wallet path:
   - `POST /api/v1/payment-sessions/{id}/wallet-instructions`
   - execute transfer in wallet
   - `POST /api/v1/payment-sessions/{id}/submit-signature`
3. QR path:
   - `POST /api/v1/payment-sessions/{id}/qr`
   - render `qrImageUrl`
   - poll `POST /api/v1/payment-sessions/{id}/detect`
4. Poll `GET /api/v1/payment-sessions/{id}/status`

Expected result: state transitions to `paid` and response includes receipt/reference metadata.

## 3) Failure path: invalid signature

- Send signed request with wrong body hash or wrong timestamp.
- Expected: `401 Unauthorized`

## 4) Failure path: revoked agent key

- Revoke key via host API `POST /api/agent-keys/revoke`.
- Retry same signed request.
- Expected: `401 Unauthorized`

## 5) Happy path: automated settlement call (JWT mode)

1. Ensure valid bearer token from nonce flow.
2. `POST /api/agent/checkout/create` with body:
   - `recipient`
   - `amount_usdc`
   - `memo`
   - optional `event_id`
3. On success receive:
   - `tx_signature`
   - `explorer_url`

## 6) Failure path: policy rejection

- Send amount > 500 USDC or invalid recipient.
- Expected: `403` policy rejection, do not retry automatically.

## 7) Failure path: no completed transaction

- Call participant status check for unknown participant.
- Expected: not-found/negative status response (do not assume paid).
