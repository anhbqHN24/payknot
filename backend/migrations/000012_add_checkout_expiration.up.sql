ALTER TABLE events
ADD COLUMN checkout_expires_at TIMESTAMPTZ;

UPDATE events
SET checkout_expires_at = COALESCE(event_date, created_at + INTERVAL '30 days')
WHERE checkout_expires_at IS NULL;

ALTER TABLE events
ALTER COLUMN checkout_expires_at SET NOT NULL;
