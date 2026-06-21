package recipe

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	recipev1 "github.com/sethlowie/dinnerwise/internal/recipe/v1"
)

// newSeededService returns a Service backed by a migrated, fixture-seeded
// temp database.
func newSeededService(t *testing.T) *Service {
	t.Helper()
	database := newTestDB(t) // migrates
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return &Service{repo: NewRepo(database)}
}

func TestServiceListRecipes(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.ListRecipes(context.Background(),
		connect.NewRequest(&recipev1.ListRecipesRequest{}))
	if err != nil {
		t.Fatalf("ListRecipes: %v", err)
	}
	if len(resp.Msg.Recipes) != 3 {
		t.Fatalf("recipes = %d, want 3", len(resp.Msg.Recipes))
	}
	// List orders by name; first fixture by name is "Sheet-Pan Chicken & Veg".
	first := resp.Msg.Recipes[0]
	if first.Id != "sheet-pan-chicken" {
		t.Fatalf("first recipe id = %q, want sheet-pan-chicken", first.Id)
	}
	if len(first.Ingredients) != 3 {
		t.Fatalf("first recipe ingredients = %d, want 3", len(first.Ingredients))
	}
}

func TestServiceGetRecipe(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.GetRecipe(context.Background(),
		connect.NewRequest(&recipev1.GetRecipeRequest{Id: "tomato-pasta"}))
	if err != nil {
		t.Fatalf("GetRecipe: %v", err)
	}
	if resp.Msg.Recipe.GetName() != "Weeknight Tomato Pasta" {
		t.Fatalf("name = %q, want Weeknight Tomato Pasta", resp.Msg.Recipe.GetName())
	}
	if len(resp.Msg.Recipe.Ingredients) != 4 {
		t.Fatalf("ingredients = %d, want 4", len(resp.Msg.Recipe.Ingredients))
	}
}

func TestServiceGetRecipeNotFound(t *testing.T) {
	svc := newSeededService(t)
	_, err := svc.GetRecipe(context.Background(),
		connect.NewRequest(&recipev1.GetRecipeRequest{Id: "does-not-exist"}))
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Fatalf("code = %v, want %v", got, connect.CodeNotFound)
	}
}
