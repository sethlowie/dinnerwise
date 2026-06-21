// Package meal is the logged-meals domain slice: schema, repo, seed, and a
// MealService mapping. Cook counts are derived from the meal_cook log.
package meal

// Meal is a logged meal with derived cook stats.
type Meal struct {
	ID          string
	Name        string
	Cuisine     string
	Rating      int
	TimesCooked int
	LastCooked  string // "" if never cooked
	RecipeID    string // "" if no linked recipe
}

// Cook is one entry in a meal's cook log.
type Cook struct {
	CookedOn string
	Note     string
}
