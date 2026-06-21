package meal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by GetByID when no meal matches the id.
var ErrNotFound = errors.New("meal not found")

// Repo is a thin hand-written SQL repository over the meal tables.
type Repo struct {
	db *sql.DB
}

func NewRepo(database *sql.DB) *Repo {
	return &Repo{db: database}
}

// List returns meals with derived cook stats. sort is "recent" (default) or
// "rating"; favoritesOnly keeps rating>=4. Sort/filter are server-side.
func (r *Repo) List(ctx context.Context, sort string, favoritesOnly bool) ([]Meal, error) {
	where := ""
	if favoritesOnly {
		where = "WHERE m.rating >= 4"
	}
	order := "ORDER BY last_cooked DESC, m.name"
	if sort == "rating" {
		order = "ORDER BY m.rating DESC, times_cooked DESC, m.name"
	}
	query := fmt.Sprintf(`
		SELECT m.id, m.name, m.cuisine, m.rating, COALESCE(m.recipe_id, ''),
		       COUNT(mc.id) AS times_cooked, COALESCE(MAX(mc.cooked_on), '') AS last_cooked
		FROM meal m
		LEFT JOIN meal_cook mc ON mc.meal_id = m.id
		%s
		GROUP BY m.id
		%s`, where, order)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query meals: %w", err)
	}
	defer rows.Close()

	meals := []Meal{}
	for rows.Next() {
		var m Meal
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Cuisine, &m.Rating, &m.RecipeID, &m.TimesCooked, &m.LastCooked,
		); err != nil {
			return nil, fmt.Errorf("scan meal: %w", err)
		}
		meals = append(meals, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meals: %w", err)
	}
	return meals, nil
}

// GetByID returns one meal with derived stats plus its recent cooks
// (newest first), or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id string) (Meal, []Cook, error) {
	var m Meal
	err := r.db.QueryRowContext(ctx, `
		SELECT m.id, m.name, m.cuisine, m.rating, COALESCE(m.recipe_id, ''),
		       COUNT(mc.id), COALESCE(MAX(mc.cooked_on), '')
		FROM meal m
		LEFT JOIN meal_cook mc ON mc.meal_id = m.id
		WHERE m.id = ?
		GROUP BY m.id`, id).
		Scan(&m.ID, &m.Name, &m.Cuisine, &m.Rating, &m.RecipeID, &m.TimesCooked, &m.LastCooked)
	if errors.Is(err, sql.ErrNoRows) {
		return Meal{}, nil, ErrNotFound
	}
	if err != nil {
		return Meal{}, nil, fmt.Errorf("query meal %q: %w", id, err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT cooked_on, note FROM meal_cook WHERE meal_id = ? ORDER BY cooked_on DESC LIMIT 5`, id)
	if err != nil {
		return Meal{}, nil, fmt.Errorf("query cooks for %q: %w", id, err)
	}
	defer rows.Close()
	cooks := []Cook{}
	for rows.Next() {
		var c Cook
		if err := rows.Scan(&c.CookedOn, &c.Note); err != nil {
			return Meal{}, nil, fmt.Errorf("scan cook: %w", err)
		}
		cooks = append(cooks, c)
	}
	if err := rows.Err(); err != nil {
		return Meal{}, nil, fmt.Errorf("iterate cooks: %w", err)
	}
	return m, cooks, nil
}
