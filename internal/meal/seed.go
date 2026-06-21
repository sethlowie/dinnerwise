package meal

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed fixtures/meals.json
var mealsFixture []byte

type seedMeal struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Cuisine     string `json:"cuisine"`
	Rating      int    `json:"rating"`
	TimesCooked int    `json:"timesCooked"`
	Last        string `json:"last"`     // ISO date "2006-01-02"
	RecipeID    string `json:"recipeId"` // "" if none
}

var cookNotes = []string{"Weeknight dinner", "Doubled the batch", "Extra garlic"}

// SeedIfEmpty loads fixture meals only when the meal table is empty. It is
// idempotent. Because cook counts are derived, it generates the full cook log:
// for a meal with timesCooked n and last date D, it writes n rows dated weekly
// back from D, attaching a rotating note to the most recent few. All inserts
// run in one transaction.
func SeedIfEmpty(database *sql.DB) error {
	var seeds []seedMeal
	if err := json.Unmarshal(mealsFixture, &seeds); err != nil {
		return fmt.Errorf("parse fixtures: %w", err)
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM meal`).Scan(&count); err != nil {
		return fmt.Errorf("count meals: %w", err)
	}
	if count > 0 {
		return nil
	}

	for _, m := range seeds {
		var recipeID any
		if m.RecipeID != "" {
			recipeID = m.RecipeID
		}
		if _, err := tx.Exec(
			`INSERT INTO meal (id, name, cuisine, rating, recipe_id) VALUES (?, ?, ?, ?, ?)`,
			m.ID, m.Name, m.Cuisine, m.Rating, recipeID,
		); err != nil {
			return fmt.Errorf("insert meal %q: %w", m.ID, err)
		}

		last, err := time.Parse("2006-01-02", m.Last)
		if err != nil {
			return fmt.Errorf("parse last for %q: %w", m.ID, err)
		}
		for i := 0; i < m.TimesCooked; i++ {
			day := last.AddDate(0, 0, -7*i).Format("2006-01-02")
			note := ""
			if i < len(cookNotes) {
				note = cookNotes[i]
			}
			if _, err := tx.Exec(
				`INSERT INTO meal_cook (meal_id, cooked_on, note) VALUES (?, ?, ?)`,
				m.ID, day, note,
			); err != nil {
				return fmt.Errorf("insert cook for %q: %w", m.ID, err)
			}
		}
	}
	return tx.Commit()
}
