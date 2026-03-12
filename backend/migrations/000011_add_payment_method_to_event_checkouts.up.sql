ALTER TABLE event_checkouts
  ADD COLUMN IF NOT EXISTS payment_method VARCHAR NOT NULL DEFAULT 'wallet';

CREATE INDEX IF NOT EXISTS event_checkouts_event_id_payment_method_idx
  ON event_checkouts(event_id, payment_method);
