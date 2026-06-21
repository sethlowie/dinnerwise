package agent

import (
	"context"
	"path/filepath"
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// newTestRepos returns recipe+meal repos backed by a migrated, seeded temp DB
// (recipes seeded before meals so meal FKs resolve).
func newTestRepos(t *testing.T) (*recipe.Repo, *meal.Repo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := recipe.Migrate(database); err != nil {
		t.Fatalf("recipe migrate: %v", err)
	}
	if err := recipe.SeedIfEmpty(database); err != nil {
		t.Fatalf("recipe seed: %v", err)
	}
	if err := meal.Migrate(database); err != nil {
		t.Fatalf("meal migrate: %v", err)
	}
	if err := meal.SeedIfEmpty(database); err != nil {
		t.Fatalf("meal seed: %v", err)
	}
	return recipe.NewRepo(database), meal.NewRepo(database)
}

func navOf(events []*agentv1.AskEvent) *agentv1.Navigate {
	for _, e := range events {
		if n, ok := e.Event.(*agentv1.AskEvent_Navigate); ok {
			return n.Navigate
		}
	}
	return nil
}

func refsOf(events []*agentv1.AskEvent) []*agentv1.Reference {
	var out []*agentv1.Reference
	for _, e := range events {
		if r, ok := e.Event.(*agentv1.AskEvent_Reference); ok {
			out = append(out, r.Reference)
		}
	}
	return out
}

func TestRespondFavorites(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "what are my favorite meals")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/meals" || nav.GetSearch()["sort"] != "rating" || nav.GetSearch()["fav"] != "1" {
		t.Fatalf("favorites navigate wrong: %+v", nav)
	}
	refs := refsOf(events)
	if len(refs) == 0 || refs[0].GetKind() != "meal" {
		t.Fatalf("favorites refs wrong: %+v", refs)
	}
}

func TestRespondTonight(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "what can I cook tonight")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/recipes" || nav.GetSearch()["pantry"] != "1" {
		t.Fatalf("tonight navigate wrong: %+v", nav)
	}
	ids := map[string]bool{}
	for _, r := range refsOf(events) {
		if r.GetKind() != "recipe" {
			t.Fatalf("tonight ref not a recipe: %+v", r)
		}
		ids[r.GetId()] = true
	}
	if !ids["tomato-pasta"] {
		t.Fatal("tonight should include in-pantry tomato-pasta")
	}
	if ids["veggie-stir-fry"] {
		t.Fatal("tonight should NOT include not-in-pantry veggie-stir-fry")
	}
}

func TestRespondQuick(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "suggest something quick")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/recipes" || nav.GetSearch()["maxMinutes"] != "30" {
		t.Fatalf("quick navigate wrong: %+v", nav)
	}
	for _, r := range refsOf(events) {
		if r.GetId() == "sheet-pan-chicken" {
			t.Fatal("quick should exclude 40-min sheet-pan-chicken")
		}
	}
}

func TestRespondIngredient(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "recipes with chicken")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/recipes" || nav.GetSearch()["ingredient"] != "chicken" {
		t.Fatalf("ingredient navigate wrong: %+v", nav)
	}
	found := false
	for _, r := range refsOf(events) {
		if r.GetId() == "sheet-pan-chicken" {
			found = true
		}
	}
	if !found {
		t.Fatal("ingredient=chicken should reference sheet-pan-chicken")
	}
}

func TestRespondNone(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "hello there")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	if navOf(events) != nil {
		t.Fatal("none should not navigate")
	}
	if len(refsOf(events)) != 0 {
		t.Fatal("none should have no references")
	}
	if _, ok := events[len(events)-1].Event.(*agentv1.AskEvent_Done); !ok {
		t.Fatal("expected Done last")
	}
}

func TestRespondOrdering(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, _ := respond(context.Background(), recipes, meals, "what are my favorites")
	// thinking(0) before tool_call before text before reference before done.
	idx := func(match func(*agentv1.AskEvent) bool) int {
		for i, e := range events {
			if match(e) {
				return i
			}
		}
		return -1
	}
	tool := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_ToolCall); return ok })
	thinking := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Thinking); return ok })
	text := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Text); return ok })
	ref := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Reference); return ok })
	done := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Done); return ok })
	if !(thinking < tool && tool < text && text < ref && ref < done) {
		t.Fatalf("ordering wrong: thinking=%d tool=%d text=%d ref=%d done=%d", thinking, tool, text, ref, done)
	}
}
