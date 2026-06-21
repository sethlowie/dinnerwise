package recipe

import (
	"database/sql"
	_ "embed"

	"github.com/sethlowie/dinnerwise/internal/db"
)

//go:embed schema.sql
var schema string

// Migrate creates the recipe tables if they do not exist. Idempotent.
func Migrate(database *sql.DB) error {
	return db.ApplySchema(database, schema)
}
