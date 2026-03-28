-- +goose Up
-- Add cycle-time and skill fields to operations (BLOC 5 — TRS & AS9100D §7.2).
-- planned_duration_seconds: copied from routing_step at operation creation.
-- actual_duration_seconds: computed at completion (completed_at - started_at).
-- required_skill: JWT role required to start this operation (AS9100D §7.2).
ALTER TABLE operations
    ADD COLUMN planned_duration_seconds INT NOT NULL DEFAULT 0,
    ADD COLUMN actual_duration_seconds  INT NOT NULL DEFAULT 0,
    ADD COLUMN required_skill           TEXT;

-- +goose Down
ALTER TABLE operations
    DROP COLUMN IF EXISTS planned_duration_seconds,
    DROP COLUMN IF EXISTS actual_duration_seconds,
    DROP COLUMN IF EXISTS required_skill;
