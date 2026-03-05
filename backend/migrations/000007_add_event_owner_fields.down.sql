DROP INDEX IF EXISTS events_owner_email_idx;
DROP INDEX IF EXISTS events_owner_wallet_idx;

ALTER TABLE "events"
DROP COLUMN "owner_email",
DROP COLUMN "owner_wallet";
