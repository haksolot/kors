-- +goose Up
-- Add NADCAP Special Process fields to operations and routing_steps (§13 — EN9100).
-- is_special_process: flags an operation/step as subject to NADCAP qualification.
-- nadcap_process_code: the specific NADCAP process identifier (e.g. 'NADCAP-WELD', 'NADCAP-NDT').
-- The interlock is enforced at the domain layer in Operation.Start().

ALTER TABLE operations
    ADD COLUMN is_special_process  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN nadcap_process_code TEXT;

ALTER TABLE routing_steps
    ADD COLUMN is_special_process  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN nadcap_process_code TEXT;

CREATE INDEX idx_operations_special_process ON operations(is_special_process) WHERE is_special_process = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_operations_special_process;

ALTER TABLE routing_steps
    DROP COLUMN IF EXISTS nadcap_process_code,
    DROP COLUMN IF EXISTS is_special_process;

ALTER TABLE operations
    DROP COLUMN IF EXISTS nadcap_process_code,
    DROP COLUMN IF EXISTS is_special_process;
