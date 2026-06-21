// Package db holds domain-agnostic SQLite plumbing: opening a connection with
// dinnerwise's pragmas and applying schema. It knows nothing about any domain.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens (creating if needed) the SQLite database at path with the pragmas
// dinnerwise relies on, then verifies the connection. Pragmas are set in the
// DSN so modernc applies them to every pooled connection.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		path,
	)
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return database, nil
}

// ApplySchema executes idempotent DDL against the database. Domain packages
// pass their embedded schema.sql here from their Migrate function.
func ApplySchema(database *sql.DB, schema string) error {
	if _, err := database.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
