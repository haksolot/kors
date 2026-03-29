-- +goose Up

-- operator_qualifications: one row per habilitation held by an operator (AS9100D §7.2).
-- Status is computed at query time from is_revoked and expires_at — never stored as a column.
-- The interlock check (StartOperation) hits the partial index idx_qual_operator_skill.
CREATE TABLE operator_qualifications (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id     UUID        NOT NULL,
    skill           TEXT        NOT NULL,   -- matches operations.required_skill
    label           TEXT        NOT NULL,
    issued_at       TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    granted_by      UUID        NOT NULL,
    certificate_url TEXT,
    is_revoked      BOOLEAN     NOT NULL DEFAULT FALSE,
    revoked_by      UUID,
    revoked_at      TIMESTAMPTZ,
    revoke_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_qual_expires_after_issued
        CHECK (expires_at > issued_at),

    -- Revocation fields must be all-set or all-null together.
    CONSTRAINT chk_qual_revoke_consistency
        CHECK (
            (is_revoked = FALSE AND revoked_by IS NULL  AND revoked_at IS NULL)
            OR
            (is_revoked = TRUE  AND revoked_by IS NOT NULL AND revoked_at IS NOT NULL)
        )
);

-- Hot path: StartOperation interlock — operator_id + skill, active only.
CREATE INDEX idx_qual_operator_skill
    ON operator_qualifications(operator_id, skill)
    WHERE is_revoked = FALSE;

-- Alert scanner: scan for qualifications expiring within N days.
CREATE INDEX idx_qual_expires_at
    ON operator_qualifications(expires_at)
    WHERE is_revoked = FALSE;

-- Audit history: list all qualifications (all statuses) for an operator.
CREATE INDEX idx_qual_operator_id
    ON operator_qualifications(operator_id);

-- +goose Down
DROP INDEX IF EXISTS idx_qual_operator_id;
DROP INDEX IF EXISTS idx_qual_expires_at;
DROP INDEX IF EXISTS idx_qual_operator_skill;
DROP TABLE IF EXISTS operator_qualifications;
