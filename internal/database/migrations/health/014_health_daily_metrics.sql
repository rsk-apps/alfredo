CREATE TABLE IF NOT EXISTS health_daily_metrics (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    date         TEXT    NOT NULL,
    metric_type  TEXT    NOT NULL,
    value        REAL    NOT NULL,
    unit         TEXT    NOT NULL,
    sleep_stages TEXT,
    created_at   TEXT    NOT NULL,
    updated_at   TEXT    NOT NULL,
    UNIQUE (date, metric_type)
);

CREATE INDEX idx_health_daily_metrics_type_date
  ON health_daily_metrics (metric_type, date);
