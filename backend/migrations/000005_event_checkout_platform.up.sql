CREATE TYPE checkout_status AS ENUM ('pending_payment', 'paid', 'approved', 'failed');

CREATE TABLE "events" (
  "id" bigserial PRIMARY KEY,
  "slug" varchar UNIQUE NOT NULL,
  "title" varchar NOT NULL,
  "description" text NOT NULL DEFAULT '',
  "event_date" timestamptz,
  "location" varchar NOT NULL DEFAULT '',
  "organizer_name" varchar NOT NULL DEFAULT '',
  "merchant_wallet" varchar NOT NULL,
  "amount_usdc" bigint NOT NULL,
  "network" varchar NOT NULL DEFAULT 'devnet',
  "status" varchar NOT NULL DEFAULT 'active',
  "created_at" timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE "invite_codes" (
  "id" bigserial PRIMARY KEY,
  "event_id" bigint NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  "code" varchar NOT NULL UNIQUE,
  "max_uses" integer NOT NULL DEFAULT 1,
  "used_count" integer NOT NULL DEFAULT 0,
  "expires_at" timestamptz,
  "status" varchar NOT NULL DEFAULT 'active',
  "created_at" timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE "event_checkouts" (
  "id" bigserial PRIMARY KEY,
  "event_id" bigint NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  "invite_code_id" bigint NOT NULL REFERENCES invite_codes(id) ON DELETE RESTRICT,
  "wallet_address" varchar NOT NULL,
  "reference" varchar NOT NULL UNIQUE,
  "signature" varchar,
  "amount" bigint NOT NULL,
  "status" checkout_status NOT NULL DEFAULT 'pending_payment',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "paid_at" timestamptz,
  "approved_at" timestamptz,
  "approved_by" varchar,
  "notes" varchar
);

CREATE INDEX ON "events" ("slug");
CREATE INDEX ON "invite_codes" ("event_id");
CREATE INDEX ON "invite_codes" ("code");
CREATE INDEX ON "event_checkouts" ("event_id");
CREATE INDEX ON "event_checkouts" ("reference");
CREATE INDEX ON "event_checkouts" ("status");
