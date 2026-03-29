-- +goose Up
CREATE TABLE tools (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serial_number       TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    description         TEXT,
    category            TEXT,
    status              TEXT NOT NULL DEFAULT 'VALID',
    last_calibration_at TIMESTAMPTZ,
    next_calibration_at TIMESTAMPTZ,
    current_cycles      INT NOT NULL DEFAULT 0,
    max_cycles          INT NOT NULL DEFAULT 0, -- 0 = unlimited
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tools_status ON tools(status);
CREATE INDEX idx_tools_next_calibration ON tools(next_calibration_at);

CREATE TABLE operation_tools (
    operation_id UUID NOT NULL REFERENCES operations(id),
    tool_id      UUID NOT NULL REFERENCES tools(id),
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (operation_id, tool_id)
);

-- +goose Down
DROP TABLE operation_tools;
DROP TABLE tools;
