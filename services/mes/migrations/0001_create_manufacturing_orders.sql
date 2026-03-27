-- +goose Up
CREATE TABLE manufacturing_orders (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reference    TEXT        NOT NULL UNIQUE,
    product_id   UUID        NOT NULL,
    quantity     INT         NOT NULL CHECK (quantity > 0),
    status       TEXT        NOT NULL DEFAULT 'planned'
                             CHECK (status IN ('planned','in_progress','completed','suspended','cancelled')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_mo_status     ON manufacturing_orders(status);
CREATE INDEX idx_mo_product_id ON manufacturing_orders(product_id);

-- +goose Down
DROP TABLE manufacturing_orders;
