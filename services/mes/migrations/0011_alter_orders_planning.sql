-- +goose Up
-- Add planning fields to manufacturing_orders (BLOC 5 — dispatch list & priority).
-- priority: 1 (lowest) to 100 (highest); defaults to 50.
-- due_date: target completion date, optional.
ALTER TABLE manufacturing_orders
    ADD COLUMN due_date  TIMESTAMPTZ,
    ADD COLUMN priority  SMALLINT NOT NULL DEFAULT 50
        CONSTRAINT priority_range CHECK (priority BETWEEN 1 AND 100);

CREATE INDEX idx_orders_dispatch ON manufacturing_orders (priority DESC, due_date ASC NULLS LAST)
    WHERE status IN ('planned', 'in_progress');

-- +goose Down
DROP INDEX IF EXISTS idx_orders_dispatch;
ALTER TABLE manufacturing_orders
    DROP COLUMN IF EXISTS due_date,
    DROP COLUMN IF EXISTS priority;
