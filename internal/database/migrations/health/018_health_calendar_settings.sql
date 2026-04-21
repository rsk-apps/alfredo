CREATE TABLE IF NOT EXISTS health_calendar_settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    google_calendar_id TEXT NOT NULL
);
