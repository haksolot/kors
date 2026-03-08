-- +goose Up
-- +goose StatementBegin
CREATE TABLE kors.modules (
    name           TEXT PRIMARY KEY,
    schema_name    TEXT NOT NULL,
    pg_username    TEXT NOT NULL,
    minio_bucket   TEXT NOT NULL,
    identity_id    UUID REFERENCES kors.identities(id) ON DELETE SET NULL,
    provisioned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    provisioned_by UUID REFERENCES kors.identities(id) ON DELETE SET NULL
);

CREATE INDEX idx_modules_identity ON kors.modules(identity_id);

-- Index de performance pour les queries soft-delete (si absent de la migration 1)
CREATE INDEX IF NOT EXISTS idx_resources_deleted_at
    ON kors.resources(deleted_at) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kors.modules;
-- +goose StatementEnd
