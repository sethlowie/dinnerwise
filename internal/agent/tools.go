package agent

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

const (
	toolSearchRecipes = "search_recipes"
	toolSearchMeals   = "search_meals"
	toolNavigate      = "navigate"
)

type toolResult struct {
	Events  []*agentv1.AskEvent
	Summary string
}

type searchRecipesArgs struct {
	Ingredient string `json:"ingredient"`
	InPantry   bool   `json:"in_pantry"`
	MaxMinutes int    `json:"max_minutes"`
}

type searchMealsArgs struct {
	FavoritesOnly bool   `json:"favorites_only"`
	Sort          string `json:"sort"`
}

type navigateArgs struct {
	To     string            `json:"to"`
	Search map[string]string `json:"search"`
}

// executeTool runs a model tool call and returns events to stream plus a compact
// JSON summary to feed back to the model.
func executeTool(ctx context.Context, recipes *recipe.Repo, meals *meal.Repo, name, argsJSON string) (toolResult, error) {
	switch name {
	case toolSearchRecipes:
		var a searchRecipesArgs
		_ = json.Unmarshal([]byte(argsJSON), &a)
		rs, err := recipes.List(ctx)
		if err != nil {
			return toolResult{}, err
		}
		if a.Ingredient != "" {
			rs = selectByIngredient(rs, a.Ingredient)
		}
		if a.InPantry {
			rs = selectInPantry(rs)
		}
		if a.MaxMinutes > 0 {
			rs = selectMaxMinutes(rs, a.MaxMinutes)
		}
		evs := recipeRefs(rs) // capped at 3, references in dock
		return toolResult{Events: evs, Summary: summarizeRecipes(rs)}, nil

	case toolSearchMeals:
		var a searchMealsArgs
		_ = json.Unmarshal([]byte(argsJSON), &a)
		sort := a.Sort
		if sort == "" {
			sort = "recent"
		}
		ms, err := meals.List(ctx, sort, a.FavoritesOnly)
		if err != nil {
			return toolResult{}, err
		}
		top := ms
		if len(top) > 3 {
			top = top[:3]
		}
		var evs []*agentv1.AskEvent
		for _, m := range top {
			evs = append(evs, referenceEvent("meal", m.ID, m.Name, mealSubtitle(m)))
		}
		return toolResult{Events: evs, Summary: summarizeMeals(ms)}, nil

	case toolNavigate:
		var a navigateArgs
		_ = json.Unmarshal([]byte(argsJSON), &a)
		return toolResult{
			Events:  []*agentv1.AskEvent{navigateEvent(a.To, a.Search)},
			Summary: fmt.Sprintf(`{"navigated":%q}`, a.To),
		}, nil
	}
	return toolResult{}, fmt.Errorf("unknown tool %q", name)
}

func summarizeRecipes(rs []recipe.Recipe) string {
	type row struct {
		ID, Name, Cuisine string
		Minutes           int
		InPantry          bool
	}
	out := make([]row, 0, len(rs))
	for _, r := range rs {
		out = append(out, row{r.ID, r.Name, r.Cuisine, r.TotalMinutes, r.InPantry})
	}
	b, _ := json.Marshal(map[string]any{"count": len(rs), "recipes": out})
	return string(b)
}

func summarizeMeals(ms []meal.Meal) string {
	type row struct {
		ID, Name    string
		Rating      int
		TimesCooked int
	}
	out := make([]row, 0, len(ms))
	for _, m := range ms {
		out = append(out, row{m.ID, m.Name, m.Rating, m.TimesCooked})
	}
	b, _ := json.Marshal(map[string]any{"count": len(ms), "meals": out})
	return string(b)
}

// toolDefs returns the function-tool definitions advertised to the model.
func toolDefs() []responses.ToolUnionParam {
	strObj := func(props map[string]any, required ...string) map[string]any {
		m := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}
	return []responses.ToolUnionParam{
		{OfFunction: &responses.FunctionToolParam{
			Name:        toolSearchRecipes,
			Description: openai.String("Search the user's recipes. Filter by ingredient (substring), in_pantry (cookable now), or max_minutes."),
			Parameters: strObj(map[string]any{
				"ingredient":  map[string]string{"type": "string"},
				"in_pantry":   map[string]string{"type": "boolean"},
				"max_minutes": map[string]string{"type": "integer"},
			}),
		}},
		{OfFunction: &responses.FunctionToolParam{
			Name:        toolSearchMeals,
			Description: openai.String("Search the user's cooked meals log. favorites_only keeps 4★+; sort is 'recent' or 'rating'."),
			Parameters: strObj(map[string]any{
				"favorites_only": map[string]string{"type": "boolean"},
				"sort":           map[string]string{"type": "string"},
			}),
		}},
		{OfFunction: &responses.FunctionToolParam{
			Name:        toolNavigate,
			Description: openai.String("Navigate the UI. to is a path like '/recipes' or '/meals'; search is string key/values like {\"pantry\":\"1\"} or {\"sort\":\"rating\",\"fav\":\"1\"}."),
			Parameters: strObj(map[string]any{
				"to":     map[string]string{"type": "string"},
				"search": map[string]any{"type": "object", "additionalProperties": map[string]string{"type": "string"}},
			}, "to"),
		}},
	}
}
