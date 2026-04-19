CREATE TABLE IF NOT EXISTS health_workout_sessions (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    activity_type        TEXT    NOT NULL,
    start_date           TEXT    NOT NULL,
    end_date             TEXT    NOT NULL,
    duration_seconds     REAL    NOT NULL,
    active_calories_kcal REAL,
    basal_calories_kcal  REAL,
    hr_avg_bpm           REAL,
    hr_min_bpm           REAL,
    hr_max_bpm           REAL,
    distance_m           REAL,
    source               TEXT    NOT NULL,
    created_at           TEXT    NOT NULL,
    updated_at           TEXT    NOT NULL,
    UNIQUE (start_date)
);

CREATE INDEX idx_health_workout_sessions_start_date
  ON health_workout_sessions (start_date);
