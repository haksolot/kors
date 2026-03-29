-- +goose Up
-- Material Lot extensions
ALTER TABLE lots ADD COLUMN status TEXT NOT NULL DEFAULT 'VALID';
ALTER TABLE lots ADD COLUMN expiry_at TIMESTAMPTZ;
ALTER TABLE lots ADD COLUMN toe_threshold_minutes INT NOT NULL DEFAULT 0;
ALTER TABLE lots ADD COLUMN toe_exposure_minutes INT NOT NULL DEFAULT 0;
ALTER TABLE lots ADD COLUMN current_workstation_id UUID;

CREATE INDEX idx_lots_status ON lots(status);
CREATE INDEX idx_lots_expiry ON lots(expiry_at);

-- Consumption tracking
CREATE TABLE material_consumptions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lot_id       UUID NOT NULL REFERENCES lots(id),
    operation_id UUID NOT NULL REFERENCES operations(id),
    quantity     INT NOT NULL CHECK (quantity > 0),
    operator_id  UUID NOT NULL,
    consumed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mat_cons_lot ON material_consumptions(lot_id);
CREATE INDEX idx_mat_cons_op ON material_consumptions(operation_id);

-- TOE tracking
CREATE TABLE toe_exposure_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lot_id      UUID NOT NULL REFERENCES lots(id),
    start_time  TIMESTAMPTZ NOT NULL,
    end_time    TIMESTAMPTZ, -- NULL if ongoing
    operator_id UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_toe_logs_lot ON toe_exposure_logs(lot_id);
CREATE INDEX idx_toe_logs_ongoing ON toe_exposure_logs(lot_id) WHERE end_time IS NULL;

-- Location / WIP tracking
CREATE TABLE location_transfers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id           UUID NOT NULL,
    entity_type         TEXT NOT NULL, -- 'LOT' or 'SERIAL'
    from_workstation_id UUID,
    to_workstation_id   UUID NOT NULL,
    transferred_by      UUID NOT NULL,
    transferred_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loc_trans_entity ON location_transfers(entity_id);

-- +goose Down
DROP TABLE location_transfers;
DROP TABLE toe_exposure_logs;
DROP TABLE material_consumptions;
ALTER TABLE lots DROP COLUMN current_workstation_id;
ALTER TABLE lots DROP COLUMN toe_exposure_minutes;
ALTER TABLE lots DROP COLUMN toe_threshold_minutes;
ALTER TABLE lots DROP COLUMN expiry_at;
ALTER TABLE lots DROP COLUMN status;
