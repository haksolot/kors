-- +goose Up
CREATE TABLE lots (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reference        TEXT        NOT NULL UNIQUE,
    product_id       UUID        NOT NULL,
    quantity         INT         NOT NULL CHECK (quantity > 0),
    material_cert_url TEXT,
    received_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lots_product_id ON lots(product_id);

-- +goose Down
DROP TABLE lots;
