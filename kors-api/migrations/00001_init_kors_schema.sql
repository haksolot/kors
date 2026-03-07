-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS kors;

CREATE TABLE kors.identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT UNIQUE,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kors.resource_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    json_schema JSONB NOT NULL DEFAULT '{}',
    transitions JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kors.resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type_id UUID NOT NULL REFERENCES kors.resource_types(id),
    state TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE kors.events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID REFERENCES kors.resources(id),
    identity_id UUID NOT NULL REFERENCES kors.identities(id),
    type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    nats_message_id UUID UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kors.revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID NOT NULL REFERENCES kors.resources(id),
    identity_id UUID NOT NULL REFERENCES kors.identities(id),
    snapshot JSONB NOT NULL DEFAULT '{}',
    file_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE kors.permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id UUID NOT NULL REFERENCES kors.identities(id),
    resource_id UUID,
    resource_type_id UUID REFERENCES kors.resource_types(id),
    action TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_resources_type ON kors.resources(type_id);
CREATE INDEX idx_resources_state ON kors.resources(state);
CREATE INDEX idx_events_resource ON kors.events(resource_id);
CREATE INDEX idx_events_nats_id ON kors.events(nats_message_id);
CREATE INDEX idx_permissions_identity ON kors.permissions(identity_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS kors CASCADE;
-- +goose StatementEnd
