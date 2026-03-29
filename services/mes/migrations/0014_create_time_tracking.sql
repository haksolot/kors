-- +goose Up
CREATE TABLE time_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_id   UUID NOT NULL,
    workstation_id UUID NOT NULL,
    operator_id    UUID NOT NULL,
    log_type       TEXT NOT NULL, -- 'SETUP' or 'RUN'
    start_time     TIMESTAMPTZ NOT NULL,
    end_time       TIMESTAMPTZ NOT NULL,
    good_qty       INT NOT NULL DEFAULT 0,
    scrap_qty      INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_time_logs_workstation ON time_logs(workstation_id, start_time, end_time);
CREATE INDEX idx_time_logs_operation ON time_logs(operation_id);

CREATE TABLE downtime_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workstation_id UUID NOT NULL,
    operation_id   UUID, -- Optional
    category       TEXT NOT NULL,
    description    TEXT,
    start_time     TIMESTAMPTZ NOT NULL,
    end_time       TIMESTAMPTZ, -- NULL means ongoing
    reported_by    UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_downtime_events_workstation ON downtime_events(workstation_id, start_time, end_time);
CREATE INDEX idx_downtime_events_ongoing ON downtime_events(workstation_id) WHERE end_time IS NULL;

-- +goose Down
DROP TABLE downtime_events;
DROP TABLE time_logs;
