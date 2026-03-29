-- +goose Up
CREATE TABLE workstations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    description   TEXT,
    capacity      INT NOT NULL DEFAULT 1,
    nominal_rate  DOUBLE PRECISION NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'AVAILABLE',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workstations_status ON workstations(status);

-- +goose Down
DROP TABLE workstations;
