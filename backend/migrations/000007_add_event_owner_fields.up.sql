ALTER TABLE "events"
ADD COLUMN "owner_email" varchar NOT NULL DEFAULT '',
ADD COLUMN "owner_wallet" varchar NOT NULL DEFAULT '';

CREATE INDEX ON "events" ("owner_email");
CREATE INDEX ON "events" ("owner_wallet");
