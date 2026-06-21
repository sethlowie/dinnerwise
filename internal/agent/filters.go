package agent

import (
	"fmt"
	"strings"

	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// selectByIngredient keeps recipes with an ingredient name containing the
// (lower-cased) substring.
func selectByIngredient(rs []recipe.Recipe, ingredient string) []recipe.Recipe {
	ingredient = strings.ToLower(ingredient)
	var out []recipe.Recipe
	for _, r := range rs {
		for _, i := range r.Ingredients {
			if strings.Contains(strings.ToLower(i.Name), ingredient) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// selectInPantry keeps recipes the user can cook now.
func selectInPantry(rs []recipe.Recipe) []recipe.Recipe {
	var out []recipe.Recipe
	for _, r := range rs {
		if r.InPantry {
			out = append(out, r)
		}
	}
	return out
}

// selectMaxMinutes keeps recipes at or under max total minutes.
func selectMaxMinutes(rs []recipe.Recipe, max int) []recipe.Recipe {
	var out []recipe.Recipe
	for _, r := range rs {
		if r.TotalMinutes <= max {
			out = append(out, r)
		}
	}
	return out
}

func recipeSubtitle(r recipe.Recipe) string {
	return fmt.Sprintf("%s · %d min", r.Cuisine, r.TotalMinutes)
}

func mealSubtitle(m meal.Meal) string {
	return fmt.Sprintf("%d★ · cooked %d×", m.Rating, m.TimesCooked)
}
