ALTER TABLE event_checkouts
  DROP COLUMN IF EXISTS participant_data;

ALTER TABLE event_checkouts
  ALTER COLUMN invite_code_id SET NOT NULL;

ALTER TABLE events
  DROP COLUMN IF EXISTS payment_methods,
  DROP COLUMN IF EXISTS participant_form_schema,
  DROP COLUMN IF EXISTS source_url,
  DROP COLUMN IF EXISTS event_source;
