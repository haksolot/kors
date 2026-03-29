-- +goose Up
CREATE TABLE control_characteristics (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    step_id          UUID NOT NULL REFERENCES routing_steps(id),
    name             TEXT NOT NULL,
    type             TEXT NOT NULL, -- 'QUANTITATIVE' or 'QUALITATIVE'
    unit             TEXT,
    nominal_value    DOUBLE PRECISION,
    upper_tolerance  DOUBLE PRECISION,
    lower_tolerance  DOUBLE PRECISION,
    is_mandatory     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ctrl_char_step ON control_characteristics(step_id);

CREATE TABLE measurements (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_id      UUID NOT NULL REFERENCES operations(id),
    characteristic_id UUID NOT NULL REFERENCES control_characteristics(id),
    value             TEXT NOT NULL,
    status            TEXT NOT NULL, -- 'PASS', 'FAIL', 'WARNING'
    operator_id       UUID NOT NULL,
    recorded_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_measurements_op ON measurements(operation_id);
CREATE INDEX idx_measurements_char ON measurements(characteristic_id, recorded_at DESC);

-- +goose Down
DROP TABLE measurements;
DROP TABLE control_characteristics;
