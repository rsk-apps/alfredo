CREATE TABLE IF NOT EXISTS treatments (
    id             TEXT PRIMARY KEY,
    pet_id         TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    dosage_amount  REAL NOT NULL,
    dosage_unit    TEXT NOT NULL,
    route          TEXT NOT NULL,
    interval_hours INTEGER NOT NULL,
    started_at     TEXT NOT NULL,
    ended_at       TEXT,
    stopped_at     TEXT,
    vet_name       TEXT,
    notes          TEXT,
    created_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS doses (
    id            TEXT PRIMARY KEY,
    treatment_id  TEXT NOT NULL REFERENCES treatments(id) ON DELETE CASCADE,
    pet_id        TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    scheduled_for TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_doses_treatment_id ON doses(treatment_id);

CREATE INDEX IF NOT EXISTS idx_doses_pet_id_scheduled ON doses(pet_id, scheduled_for)