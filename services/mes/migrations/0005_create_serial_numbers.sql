-- +goose Up
CREATE TABLE serial_numbers (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sn         TEXT        NOT NULL UNIQUE,
    lot_id     UUID        REFERENCES lots(id),
    product_id UUID        NOT NULL,
    of_id      UUID        NOT NULL REFERENCES manufacturing_orders(id),
    status     TEXT        NOT NULL DEFAULT 'produced'
                           CHECK (status IN ('produced', 'released', 'scrapped')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sn_lot_id     ON serial_numbers(lot_id);
CREATE INDEX idx_sn_product_id ON serial_numbers(product_id);
CREATE INDEX idx_sn_of_id      ON serial_numbers(of_id);
CREATE INDEX idx_sn_status     ON serial_numbers(status);

-- +goose Down
DROP TABLE serial_numbers;
