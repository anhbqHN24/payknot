# SDK Example (Python)

```python
import os, time, json, base64, hashlib
import requests
from nacl.signing import SigningKey

BASE = "https://pay.crea8r.xyz"
AGENT_ID = os.environ["AGENT_ID"]
SK = SigningKey(base64.b64decode(os.environ["AGENT_SK_B64"]))

def sign_headers(method: str, path: str, body: str):
    ts = str(int(time.time()))
    body_hash = hashlib.sha256(body.encode("utf-8")).hexdigest()
    canonical = f"{method}\n{path}\n{ts}\n{body_hash}".encode("utf-8")
    sig = SK.sign(canonical).signature
    return {
        "X-Agent-Id": AGENT_ID,
        "X-Agent-Timestamp": ts,
        "X-Agent-Signature": base64.b64encode(sig).decode("utf-8"),
    }

path = "/api/v1/payment-sessions"
payload = {
    "slug": "web3-night",
    "paymentMethod": "qr",
    "participantData": {"email": "a@b.com"}
}
body = json.dumps(payload, separators=(",", ":"))
headers = {
    "Content-Type": "application/json",
    "Idempotency-Key": f"create-{int(time.time())}",
    **sign_headers("POST", path, body)
}
r = requests.post(BASE + path, headers=headers, data=body, timeout=20)
print(r.status_code, r.text)
```
