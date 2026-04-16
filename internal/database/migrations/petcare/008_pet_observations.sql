CREATE TABLE IF NOT EXISTS pet_observations (
    id          TEXT PRIMARY KEY,
    pet_id      TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    observed_at TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pet_observations_pet_observed_at
    ON pet_observations(pet_id, observed_at);
