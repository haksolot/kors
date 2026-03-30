-- +goose Up
-- Immutable audit trail (§13 — EN9100 conformité et auditabilité).
-- This table records every state-changing action in the MES: who did what, when, on which entity.
-- Entries are NEVER updated or deleted — enforced by revocation of UPDATE/DELETE privileges
-- on this table in production (see infra/postgres/init_privileges.sql).
-- The application layer appends entries via TxOps.AppendAuditEntry within the same DB transaction
-- as the business mutation, guaranteeing consistency (ADR-004).

CREATE TABLE audit_trail (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       UUID        NOT NULL,                        -- user who performed the action
    actor_role     TEXT        NOT NULL,                        -- JWT role at the time of the action
    action         TEXT        NOT NULL,                        -- AuditAction enum value
    entity_type    TEXT        NOT NULL,                        -- 'manufacturing_order', 'operation', etc.
    entity_id      UUID        NOT NULL,                        -- UUID of the affected entity
    workstation_id UUID,                                        -- optional originating workstation
    notes          TEXT,                                        -- free-text context (e.g. suspension reason)
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Queries by entity (most common: "give me the full history of this OF / this SN")
CREATE INDEX idx_audit_entity ON audit_trail(entity_id, created_at DESC);
-- Queries by actor (compliance: "what did this operator do?")
CREATE INDEX idx_audit_actor ON audit_trail(actor_id, created_at DESC);
-- Queries by action type (analysis: "all FAI approvals this month")
CREATE INDEX idx_audit_action ON audit_trail(action, created_at DESC);

-- +goose Down
DROP TABLE audit_trail;
