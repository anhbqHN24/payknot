DROP INDEX IF EXISTS event_checkouts_status_idx;
DROP INDEX IF EXISTS event_checkouts_reference_idx;
DROP INDEX IF EXISTS event_checkouts_event_id_idx;
DROP INDEX IF EXISTS invite_codes_code_idx;
DROP INDEX IF EXISTS invite_codes_event_id_idx;
DROP INDEX IF EXISTS events_slug_idx;

DROP TABLE IF EXISTS "event_checkouts";
DROP TABLE IF EXISTS "invite_codes";
DROP TABLE IF EXISTS "events";
DROP TYPE IF EXISTS checkout_status;
