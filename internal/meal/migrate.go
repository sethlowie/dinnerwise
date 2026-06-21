package meal

import (
	"database/sql"
	_ "embed"

	"github.com/sethlowie/dinnerwise/internal/db"
)

//go:embed schema.sql
var schema string

// Migrate creates the meal tables if they do not exist. Idempotent. Requires
// the recipe table to already exist (meal.recipe_id references it).
func Migrate(database *sql.DB) error {
	return db.ApplySchema(database, schema)
}
