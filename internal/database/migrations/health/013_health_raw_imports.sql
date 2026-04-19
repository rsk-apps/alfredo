CREATE TABLE IF NOT EXISTS health_raw_imports (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    import_type  TEXT    NOT NULL,  -- 'metrics' or 'workouts'
    payload      TEXT    NOT NULL,  -- full Apple Health Exporter JSON blob
    imported_at  TEXT    NOT NULL,  -- RFC3339 timestamp of import
    created_at   TEXT    NOT NULL
);
