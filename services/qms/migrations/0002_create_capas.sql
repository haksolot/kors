-- +goose Up
-- CAPA (Corrective And Preventive Action) linked to a NonConformity.
CREATE TABLE capas (
    id           UUID        PRIMARY KEY,
    nc_id        UUID        NOT NULL REFERENCES non_conformities(id) ON DELETE CASCADE,
    action_type  TEXT        NOT NULL
        CONSTRAINT capa_action_type_check CHECK (action_type IN ('corrective','preventive')),
    description  TEXT        NOT NULL,
    owner_id     TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'open'
        CONSTRAINT capa_status_check CHECK (status IN ('open','in_progress','completed','cancelled')),
    due_date     TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_capas_nc_id  ON capas (nc_id);
CREATE INDEX idx_capas_status ON capas (status);

-- +goose Down
DROP TABLE IF EXISTS capas;
