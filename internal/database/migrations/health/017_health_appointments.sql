CREATE TABLE IF NOT EXISTS health_appointments (
    id                       TEXT PRIMARY KEY,
    specialty                TEXT NOT NULL,
    scheduled_at             TEXT NOT NULL,
    doctor                   TEXT,
    notes                    TEXT,
    google_calendar_event_id TEXT NOT NULL DEFAULT '',
    created_at               TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_health_appointments_scheduled_at ON health_appointments(scheduled_at);
