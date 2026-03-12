ALTER TABLE events
  ADD COLUMN IF NOT EXISTS event_source VARCHAR NOT NULL DEFAULT 'custom',
  ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS participant_form_schema JSONB NOT NULL DEFAULT '[{"id":"name","label":"Name","type":"text","required":true},{"id":"email","label":"Email","type":"email","required":true}]'::jsonb,
  ADD COLUMN IF NOT EXISTS payment_methods JSONB NOT NULL DEFAULT '{"wallet":true,"qr":true}'::jsonb;

ALTER TABLE event_checkouts
  ALTER COLUMN invite_code_id DROP NOT NULL;

ALTER TABLE event_checkouts
  ADD COLUMN IF NOT EXISTS participant_data JSONB NOT NULL DEFAULT '{}'::jsonb;
