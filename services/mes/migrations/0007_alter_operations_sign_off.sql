-- +goose Up
-- Add hold-point / sign-off fields to operations (AS9100D §8.6).
-- requires_sign_off: when true, Complete() moves the op to pending_sign_off.
-- signed_off_by / signed_off_at: audit trail of the quality_inspector sign-off.
-- instructions_url: optional reference to a work instruction document in MinIO.
ALTER TABLE operations
    ADD COLUMN requires_sign_off  BOOLEAN    NOT NULL DEFAULT FALSE,
    ADD COLUMN signed_off_by      UUID,
    ADD COLUMN signed_off_at      TIMESTAMPTZ,
    ADD COLUMN instructions_url   TEXT;

-- +goose Down
ALTER TABLE operations
    DROP COLUMN IF EXISTS requires_sign_off,
    DROP COLUMN IF EXISTS signed_off_by,
    DROP COLUMN IF EXISTS signed_off_at,
    DROP COLUMN IF EXISTS instructions_url;
