-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS kors;

-- 1. Identities: Actors of the system
CREATE TABLE kors.identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT UNIQUE, -- Keycloak ID or system identifier
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- 'user', 'service', 'system'
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Resource Types: Registry of types with validation and lifecycle rules
CREATE TABLE kors.resource_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL, -- e.g., 'tool', 'order', 'part'
    description TEXT,
    json_schema JSONB NOT NULL DEFAULT '{}',
    transitions JSONB NOT NULL DEFAULT '{}', -- State machine definition
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Resources: Universal index of business entities
CREATE TABLE kors.resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type_id UUID NOT NULL REFERENCES kors.resource_types(id),
    state TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ -- Soft delete support
);

-- 4. Events: Immutable audit log
CREATE TABLE kors.events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID REFERENCES kors.resources(id),
    identity_id UUID NOT NULL REFERENCES kors.identities(id),
    type TEXT NOT NULL, -- e.g., 'resource.created', 'resource.state_changed'
    payload JSONB NOT NULL DEFAULT '{}',
    nats_message_id UUID UNIQUE, -- For idempotency
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. Revisions: Versioned snapshots
CREATE TABLE kors.revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID NOT NULL REFERENCES kors.resources(id),
    identity_id UUID NOT NULL REFERENCES kors.identities(id),
    snapshot JSONB NOT NULL DEFAULT '{}',
    file_path TEXT, -- MinIO reference if applicable
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 6. Permissions: Generic RBAC
CREATE TABLE kors.permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id UUID NOT NULL REFERENCES kors.identities(id),
    resource_id UUID, -- NULL if global permission, or UUID of a specific resource
    resource_type_id UUID REFERENCES kors.resource_types(id), -- NULL if global or specific resource
    action TEXT NOT NULL, -- 'read', 'write', 'transition', etc.
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for performance
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
