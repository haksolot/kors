-- +goose Up
CREATE TABLE alerts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category         TEXT NOT NULL, -- 'MACHINE', 'QUALITY', 'PLANNING', 'LOGISTICS'
    level            TEXT NOT NULL, -- 'L1_SUPERVISOR', 'L2_MANAGER', 'L3_DIRECTOR'
    status           TEXT NOT NULL, -- 'ACTIVE', 'ACKNOWLEDGED', 'RESOLVED'
    workstation_id   UUID,
    operation_id     UUID,
    message          TEXT NOT NULL,
    escalation_count INT NOT NULL DEFAULT 0,
    acknowledged_by  UUID,
    acknowledged_at  TIMESTAMPTZ,
    resolved_by      UUID,
    resolved_at      TIMESTAMPTZ,
    resolution_notes TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alerts_status ON alerts(status) WHERE status != 'RESOLVED';
CREATE INDEX idx_alerts_level ON alerts(level);

-- +goose Down
DROP TABLE alerts;
