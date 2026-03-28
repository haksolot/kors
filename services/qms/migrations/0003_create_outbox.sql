-- +goose Up
-- Transactional outbox for QMS domain events (ADR-004).
CREATE TABLE outbox (
    id           BIGSERIAL   PRIMARY KEY,
    event_type   TEXT        NOT NULL,
    subject      TEXT        NOT NULL,
    payload      BYTEA       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unpublished ON outbox (id) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS outbox;
