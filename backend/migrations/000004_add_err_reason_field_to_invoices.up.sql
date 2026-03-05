ALTER TYPE invoice_status ADD VALUE 'error';
ALTER TABLE "invoice"
ADD COLUMN "err_reason" varchar;