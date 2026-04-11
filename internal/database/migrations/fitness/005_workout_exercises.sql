CREATE TABLE IF NOT EXISTS fitness_workout_exercises (
    id         TEXT    PRIMARY KEY,
    workout_id TEXT    NOT NULL REFERENCES fitness_workouts(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    equipment  TEXT,
    order_idx  INTEGER NOT NULL,
    UNIQUE(workout_id, order_idx)
);

CREATE INDEX IF NOT EXISTS idx_fitness_workout_exercises_workout_id
    ON fitness_workout_exercises(workout_id);

CREATE TABLE IF NOT EXISTS fitness_workout_sets (
    id            TEXT    PRIMARY KEY,
    exercise_id   TEXT    NOT NULL REFERENCES fitness_workout_exercises(id) ON DELETE CASCADE,
    set_number    INTEGER NOT NULL,
    reps          INTEGER,
    weight_kg     REAL,
    duration_secs INTEGER,
    notes         TEXT,
    UNIQUE(exercise_id, set_number)
)
