package agent

import (
	"testing"

	"github.com/sethlowie/dinnerwise/internal/recipe"
)

func r(id string, min int, pantry bool, ings ...string) recipe.Recipe {
	var ri []recipe.RecipeIngredient
	for _, n := range ings {
		ri = append(ri, recipe.RecipeIngredient{Name: n})
	}
	return recipe.Recipe{ID: id, TotalMinutes: min, InPantry: pantry, Ingredients: ri}
}

func ids(rs []recipe.Recipe) []string {
	out := []string{}
	for _, x := range rs {
		out = append(out, x.ID)
	}
	return out
}

func TestSelectByIngredient(t *testing.T) {
	rs := []recipe.Recipe{r("a", 0, false, "Chicken Thigh"), r("b", 0, false, "Tofu")}
	got := ids(selectByIngredient(rs, "chicken"))
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}

func TestSelectInPantry(t *testing.T) {
	rs := []recipe.Recipe{r("a", 0, true), r("b", 0, false)}
	if got := ids(selectInPantry(rs)); len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}

func TestSelectMaxMinutes(t *testing.T) {
	rs := []recipe.Recipe{r("a", 25, false), r("b", 40, false)}
	if got := ids(selectMaxMinutes(rs, 30)); len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}
