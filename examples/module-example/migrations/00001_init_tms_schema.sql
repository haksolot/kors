-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS tms;

CREATE TABLE IF NOT EXISTS tms.tools (
    id UUID PRIMARY KEY,
    serial_number TEXT NOT NULL,
    model TEXT NOT NULL,
    diameter DOUBLE PRECISION,
    length DOUBLE PRECISION,
    last_maintenance_at TIMESTAMPTZ,
    location TEXT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tms.tools;
-- +goose StatementEnd
