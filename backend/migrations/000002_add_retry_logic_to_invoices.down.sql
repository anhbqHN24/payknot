-- Dropping the 'failed' enum value is a complex and potentially data-destructive
-- operation that is not handled automatically. This down migration only removes
-- the columns associated with the retry logic.
ALTER TABLE "invoice"
DROP COLUMN "retry_count",
DROP COLUMN "last_retry_at";
