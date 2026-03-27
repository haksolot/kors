-- +goose Up
-- Transactional outbox table for MES events (ADR-004).
-- Events are written here in the same transaction as business data,
-- then published to NATS JetStream by the outbox worker goroutine.
CREATE TABLE outbox (
    id           BIGSERIAL   PRIMARY KEY,
    event_type   TEXT        NOT NULL,          -- human-readable, e.g. "of.created"
    subject      TEXT        NOT NULL,          -- NATS target subject, e.g. "kors.mes.of.created"
    payload      BYTEA       NOT NULL,          -- Protobuf-encoded message (proto.Marshal)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ                    -- NULL = not yet published
);

-- Partial index for efficient polling of unpublished events only.
CREATE INDEX idx_outbox_unpublished ON outbox(id) WHERE published_at IS NULL;

-- +goose Down
DROP INDEX idx_outbox_unpublished;
DROP TABLE outbox;
