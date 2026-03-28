-- +goose Up
CREATE TABLE genealogy (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_sn_id UUID        NOT NULL REFERENCES serial_numbers(id),
    child_sn_id  UUID        NOT NULL REFERENCES serial_numbers(id),
    of_id        UUID        NOT NULL REFERENCES manufacturing_orders(id),
    operation_id UUID        REFERENCES operations(id),
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_genealogy_parent_child UNIQUE (parent_sn_id, child_sn_id)
);

CREATE INDEX idx_genealogy_parent ON genealogy(parent_sn_id);
CREATE INDEX idx_genealogy_child  ON genealogy(child_sn_id);

-- +goose Down
DROP TABLE genealogy;
