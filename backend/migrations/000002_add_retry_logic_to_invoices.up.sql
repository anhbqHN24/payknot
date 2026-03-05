ALTER TYPE invoice_status ADD VALUE 'failed';
ALTER TABLE "invoice"
ADD COLUMN "retry_count" integer NOT NULL DEFAULT 0,
ADD COLUMN "last_retry_at" timestamptz;
