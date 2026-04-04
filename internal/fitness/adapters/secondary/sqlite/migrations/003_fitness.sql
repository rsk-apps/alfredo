CREATE TABLE IF NOT EXISTS fitness_profiles (
    id         TEXT PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    birth_date TEXT NOT NULL,
    gender     TEXT NOT NULL,
    height_cm  REAL NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS fitness_workouts (
    id               TEXT PRIMARY KEY,
    external_id      TEXT NOT NULL,
    type             TEXT NOT NULL,
    started_at       TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    active_calories  REAL NOT NULL,
    total_calories   REAL NOT NULL,
    distance_meters  REAL,
    avg_pace_sec_per_km REAL,
    avg_heart_rate   REAL,
    max_heart_rate   REAL,
    hr_zone1_pct     REAL,
    hr_zone2_pct     REAL,
    hr_zone3_pct     REAL,
    hr_zone4_pct     REAL,
    hr_zone5_pct     REAL,
    source           TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    UNIQUE(external_id, source)
);

CREATE INDEX IF NOT EXISTS idx_fitness_workouts_started_at ON fitness_workouts(started_at);

CREATE TABLE IF NOT EXISTS fitness_body_snapshots (
    id          TEXT PRIMARY KEY,
    date        TEXT NOT NULL UNIQUE,
    weight_kg   REAL,
    waist_cm    REAL,
    hip_cm      REAL,
    neck_cm     REAL,
    body_fat_pct REAL,
    photo_path  TEXT,
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_fitness_body_snapshots_date ON fitness_body_snapshots(date);

CREATE TABLE IF NOT EXISTS fitness_goals (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT,
    target_value REAL,
    target_unit  TEXT,
    deadline     TEXT,
    achieved_at  TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
)
