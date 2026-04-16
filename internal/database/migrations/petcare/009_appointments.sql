CREATE TABLE IF NOT EXISTS appointments (
    id TEXT PRIMARY KEY,
    pet_id TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN ('vet', 'grooming', 'other')),
    scheduled_at TEXT NOT NULL,
    provider TEXT,
    location TEXT,
    notes TEXT,
    google_calendar_event_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_appointments_pet_id ON appointments(pet_id);
CREATE INDEX IF NOT EXISTS idx_appointments_scheduled_at ON appointments(pet_id, scheduled_at)
