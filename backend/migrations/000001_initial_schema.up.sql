CREATE TYPE invoice_status AS ENUM ('pending', 'paid', 'expired');

CREATE TABLE "invoice" (
  "id" bigserial PRIMARY KEY,
  "wallet_address" varchar NOT NULL,
  "reference" varchar UNIQUE NOT NULL,
  "signature" varchar,
  "amount" bigint NOT NULL,
  "status" invoice_status NOT NULL DEFAULT 'pending',
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE VIEW "paid_wallet" AS
SELECT DISTINCT "wallet_address", MIN("created_at") as "created_at", "id" as "invoice_id"
FROM "invoice"
WHERE "status" = 'paid'
GROUP BY "wallet_address", "invoice_id";

CREATE INDEX ON "invoice" ("wallet_address");
CREATE INDEX ON "invoice" ("reference");
CREATE INDEX ON "invoice" ("status");
