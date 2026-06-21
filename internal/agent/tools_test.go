package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

func seededRepos(t *testing.T) (*recipe.Repo, *meal.Repo) {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := recipe.Migrate(database); err != nil {
		t.Fatal(err)
	}
	if err := recipe.SeedIfEmpty(database); err != nil {
		t.Fatal(err)
	}
	if err := meal.Migrate(database); err != nil {
		t.Fatal(err)
	}
	if err := meal.SeedIfEmpty(database); err != nil {
		t.Fatal(err)
	}
	return recipe.NewRepo(database), meal.NewRepo(database)
}

func TestExecuteSearchRecipesIngredient(t *testing.T) {
	recipes, meals := seededRepos(t)
	res, err := executeTool(context.Background(), recipes, meals,
		toolSearchRecipes, `{"ingredient":"chicken"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) == 0 {
		t.Fatal("expected reference events")
	}
	if !strings.Contains(strings.ToLower(res.Summary), "chicken") &&
		!json.Valid([]byte(res.Summary)) {
		t.Fatalf("summary should be valid JSON: %q", res.Summary)
	}
}

func TestExecuteNavigate(t *testing.T) {
	recipes, meals := seededRepos(t)
	res, err := executeTool(context.Background(), recipes, meals,
		toolNavigate, `{"to":"/recipes","search":{"pantry":"1"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) != 1 || res.Events[0].GetNavigate() == nil {
		t.Fatalf("expected one navigate event, got %#v", res.Events)
	}
	if res.Events[0].GetNavigate().GetTo() != "/recipes" {
		t.Fatalf("navigate.to = %q", res.Events[0].GetNavigate().GetTo())
	}
}
