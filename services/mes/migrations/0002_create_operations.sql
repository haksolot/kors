-- +goose Up
CREATE TABLE operations (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    of_id        UUID        NOT NULL REFERENCES manufacturing_orders(id) ON DELETE CASCADE,
    step_number  INT         NOT NULL CHECK (step_number > 0),
    name         TEXT        NOT NULL,
    operator_id  UUID,
    status       TEXT        NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending','in_progress','completed','skipped')),
    skip_reason  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    CONSTRAINT uq_operations_of_step UNIQUE (of_id, step_number)
);

CREATE INDEX idx_ops_of_id     ON operations(of_id);
CREATE INDEX idx_ops_status    ON operations(status);
CREATE INDEX idx_ops_operator  ON operations(operator_id) WHERE operator_id IS NOT NULL;

-- +goose Down
DROP TABLE operations;
