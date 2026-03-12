DROP INDEX IF EXISTS event_checkouts_event_id_payment_method_idx;

ALTER TABLE event_checkouts
  DROP COLUMN IF EXISTS payment_method;
