// Package recipe is a self-contained domain slice: it owns its schema, seed
// data, and a hand-written SQL repository. Repo structs are plain Go and are
// mapped to proto in the service layer if a service is ever exposed.
package recipe

// Recipe is a recipe with its assembled ingredient lines.
type Recipe struct {
	ID           string
	Name         string
	Instructions string
	Servings     int
	TotalMinutes int
	Ingredients  []RecipeIngredient
}

// RecipeIngredient is one ingredient line on a recipe (ingredient name joined
// in for convenience).
type RecipeIngredient struct {
	IngredientID string
	Name         string
	Quantity     float64
	Unit         string
}
