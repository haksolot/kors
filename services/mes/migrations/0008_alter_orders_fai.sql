-- +goose Up
-- Add First Article Inspection (FAI) fields to manufacturing_orders (AS9100D §8.6).
-- is_fai: marks the order as a FAI order requiring quality_manager approval.
-- fai_approved_by / fai_approved_at: audit trail of the approval.
ALTER TABLE manufacturing_orders
    ADD COLUMN is_fai           BOOLEAN    NOT NULL DEFAULT FALSE,
    ADD COLUMN fai_approved_by  UUID,
    ADD COLUMN fai_approved_at  TIMESTAMPTZ;

-- +goose Down
ALTER TABLE manufacturing_orders
    DROP COLUMN IF EXISTS is_fai,
    DROP COLUMN IF EXISTS fai_approved_by,
    DROP COLUMN IF EXISTS fai_approved_at;
