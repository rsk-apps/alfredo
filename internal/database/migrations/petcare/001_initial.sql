CREATE TABLE IF NOT EXISTS pets (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    species         TEXT NOT NULL,
    breed           TEXT,
    birth_date      TEXT,
    weight_kg       REAL,
    daily_food_grams REAL,
    photo_path      TEXT,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS vaccines (
    id              TEXT PRIMARY KEY,
    pet_id          TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    administered_at TEXT NOT NULL,
    next_due_at     TEXT,
    vet_name        TEXT,
    batch_number    TEXT,
    notes           TEXT
);
