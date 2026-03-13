# SDK Example (TypeScript)

```ts
import nacl from "tweetnacl";
import { createHash } from "crypto";

function sha256Hex(body: string) {
  return createHash("sha256").update(body).digest("hex");
}

function signRequest(opts: {
  method: string;
  path: string;
  body: string;
  agentId: string;
  secretKey: Uint8Array; // ed25519 private key
}) {
  const ts = Math.floor(Date.now() / 1000).toString();
  const canonical = `${opts.method}\n${opts.path}\n${ts}\n${sha256Hex(opts.body)}`;
  const sig = nacl.sign.detached(new TextEncoder().encode(canonical), opts.secretKey);
  return {
    "X-Agent-Id": opts.agentId,
    "X-Agent-Timestamp": ts,
    "X-Agent-Signature": Buffer.from(sig).toString("base64"),
  };
}

// create v1 payment session
const body = JSON.stringify({ slug: "web3-night", paymentMethod: "qr", participantData: { email: "a@b.com" } });
const path = "/api/v1/payment-sessions";
const headers = {
  "Content-Type": "application/json",
  "Idempotency-Key": `create-${Date.now()}`,
  ...signRequest({ method: "POST", path, body, agentId: process.env.AGENT_ID!, secretKey: Buffer.from(process.env.AGENT_SK!, "base64") }),
};
const res = await fetch(`https://pay.crea8r.xyz${path}`, { method: "POST", headers, body });
const session = await res.json();
```
