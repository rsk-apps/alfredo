package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/petcare/001_initial.sql
var migration001 string

//go:embed migrations/petcare/002_treatments.sql
var migration002 string

//go:embed migrations/fitness/003_fitness.sql
var migration003 string

//go:embed migrations/fitness/004_body_snapshot_measurements.sql
var migration004 string

//go:embed migrations/fitness/005_workout_exercises.sql
var migration005 string

//go:embed migrations/petcare/006_google_calendar.sql
var migration006 string

//go:embed migrations/petcare/007_vaccine_next_due_calendar.sql
var migration007 string

//go:embed migrations/petcare/008_pet_observations.sql
var migration008 string

//go:embed migrations/petcare/009_appointments.sql
var migration009 string

//go:embed migrations/petcare/010_supplies.sql
var migration010 string

//go:embed migrations/petcare/011_agent_invocations.sql
var migration011 string

//go:embed migrations/health/012_health_profiles.sql
var migration012 string

//go:embed migrations/health/013_health_raw_imports.sql
var migration013 string

//go:embed migrations/health/014_health_daily_metrics.sql
var migration014 string

//go:embed migrations/health/015_health_workout_sessions.sql
var migration015 string

//go:embed migrations/health/016_health_profile_calendar.sql
var migration016 string

//go:embed migrations/health/017_health_appointments.sql
var migration017 string

//go:embed migrations/health/018_health_calendar_settings.sql
var migration018 string

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite pragma foreign_keys: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations := []struct {
		version string
		sql     string
	}{
		{"001_initial", migration001},
		{"002_treatments", migration002},
		{"003_fitness", migration003},
		{"004_body_snapshot_measurements", migration004},
		{"005_workout_exercises", migration005},
		{"006_google_calendar", migration006},
		{"007_vaccine_next_due_calendar", migration007},
		{"008_pet_observations", migration008},
		{"009_appointments", migration009},
		{"010_supplies", migration010},
		{"011_agent_invocations", migration011},
		{"012_health_profiles", migration012},
		{"013_health_raw_imports", migration013},
		{"014_health_daily_metrics", migration014},
		{"015_health_workout_sessions", migration015},
		{"016_health_profile_calendar", migration016},
		{"017_health_appointments", migration017},
		{"018_health_calendar_settings", migration018},
	}

	for _, m := range migrations {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", m.version, err)
		}
		if count > 0 {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", m.version, err)
		}
		stmts := splitSQL(m.sql)
		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %s (stmt: %q): %w", m.version, stmt, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.version, err)
		}
	}
	return nil
}

type Checker struct {
	db *sql.DB
}

func NewChecker(db *sql.DB) *Checker {
	return &Checker{db: db}
}

func (c *Checker) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

func splitSQL(script string) []string {
	var out []string
	for _, s := range strings.Split(script, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
