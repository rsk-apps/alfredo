package sqlite

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed migrations/003_fitness.sql
var migration003 string

// Migrate applies the fitness module migration to an existing open *sql.DB.
// It uses the same schema_migrations table as the petcare migrations.
// Safe to call multiple times — skips already-applied versions.
func Migrate(db *sql.DB) error {
	migrations := []struct {
		version string
		sql     string
	}{
		{"003_fitness", migration003},
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
		for _, stmt := range splitSQL(m.sql) {
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
