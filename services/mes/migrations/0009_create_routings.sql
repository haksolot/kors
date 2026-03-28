-- +goose Up
-- Routing template: defines the ordered production steps for a product.
CREATE TABLE routings (
    id         UUID        PRIMARY KEY,
    product_id UUID        NOT NULL,
    version    INT         NOT NULL,
    name       TEXT        NOT NULL,
    is_active  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, version)
);

CREATE TABLE routing_steps (
    id                      UUID    PRIMARY KEY,
    routing_id              UUID    NOT NULL REFERENCES routings(id) ON DELETE CASCADE,
    step_number             INT     NOT NULL,
    name                    TEXT    NOT NULL,
    planned_duration_seconds INT    NOT NULL DEFAULT 0,
    required_skill          TEXT,
    instructions_url        TEXT,
    requires_sign_off       BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (routing_id, step_number)
);

-- +goose Down
DROP TABLE IF EXISTS routing_steps;
DROP TABLE IF EXISTS routings;
