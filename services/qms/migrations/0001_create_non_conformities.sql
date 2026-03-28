-- +goose Up
-- NonConformity aggregate (AS9100D §8.7).
-- operation_id is a unique dedup key: one NC per MES operation.
CREATE TABLE non_conformities (
    id                 UUID        PRIMARY KEY,
    operation_id       TEXT        NOT NULL UNIQUE,  -- MES operation UUID (dedup)
    of_id              TEXT        NOT NULL,
    defect_code        TEXT        NOT NULL,
    description        TEXT        NOT NULL DEFAULT '',
    affected_quantity  INT         NOT NULL CHECK (affected_quantity >= 1),
    serial_numbers     TEXT[]      NOT NULL DEFAULT '{}',
    declared_by        TEXT        NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'open'
        CONSTRAINT nc_status_check CHECK (status IN ('open','under_analysis','pending_disposition','closed')),
    disposition        TEXT
        CONSTRAINT nc_disposition_check CHECK (disposition IN ('rework','scrap','use_as_is','return_to_supplier')),
    closed_by          TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at          TIMESTAMPTZ
);

CREATE INDEX idx_nc_status ON non_conformities (status);
CREATE INDEX idx_nc_of_id  ON non_conformities (of_id);

-- +goose Down
DROP TABLE IF EXISTS non_conformities;
