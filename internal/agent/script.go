package agent

import (
	"context"
	"fmt"
	"strings"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// knownIngredients are the keywords the scripted (no-LLM) agent recognizes.
var knownIngredients = []string{
	"chicken", "tomato", "tofu", "garlic", "broccoli", "rice", "pasta",
}

// intentFrom maps user text to a scenario (keyword-scripted, first match wins).
// For the ingredient scenario it also returns the matched ingredient keyword.
func intentFrom(text string) (intent, ingredient string) {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "favorite") || strings.Contains(t, "best"):
		return "favorites", ""
	case strings.Contains(t, "quick") || strings.Contains(t, "fast"):
		return "quick", ""
	case strings.Contains(t, "tonight") || strings.Contains(t, "cook") ||
		strings.Contains(t, "dinner") || strings.Contains(t, "have"):
		return "tonight", ""
	}
	for _, ing := range knownIngredients {
		if strings.Contains(t, ing) {
			return "ingredient", ing
		}
	}
	return "none", ""
}

// respond is the scripted agent: it queries the real repos for the matched
// intent and returns the event sequence to stream. Not pure (DB access) but
// deterministic given the data. The real LLM agent would replace this, calling
// the same repos as tools.
func respond(ctx context.Context, recipes *recipe.Repo, meals *meal.Repo, text string) ([]*agentv1.AskEvent, error) {
	intent, ing := intentFrom(text)
	switch intent {
	case "favorites":
		ms, err := meals.List(ctx, "rating", true)
		if err != nil {
			return nil, err
		}
		evs := []*agentv1.AskEvent{
			thinkingEvent("Opening your Meals log…"),
			thinkingEvent("Sorting by your ratings…"),
			toolCallEvent("search_meals", "favorites · rating>=4"),
		}
		evs = append(evs, textChunks(fmt.Sprintf(
			"You've rated %d meals four stars or higher. Here are your favorites.", len(ms)))...)
		evs = append(evs, navigateEvent("/meals", map[string]string{"sort": "rating", "fav": "1"}))
		top := ms
		if len(top) > 3 {
			top = top[:3]
		}
		for _, m := range top {
			evs = append(evs, referenceEvent("meal", m.ID, m.Name, mealSubtitle(m)))
		}
		return append(evs, doneEvent()), nil

	case "tonight":
		rs, err := recipes.List(ctx)
		if err != nil {
			return nil, err
		}
		have := selectInPantry(rs)
		evs := []*agentv1.AskEvent{
			thinkingEvent("Checking what's in your pantry…"),
			thinkingEvent("Matching recipes you can make now…"),
			toolCallEvent("search_recipes", "in_pantry=true"),
		}
		evs = append(evs, textChunks(fmt.Sprintf(
			"You can cook %d recipes tonight without a shop.", len(have)))...)
		evs = append(evs, navigateEvent("/recipes", map[string]string{"pantry": "1"}))
		evs = append(evs, recipeRefs(have)...)
		return append(evs, doneEvent()), nil

	case "quick":
		rs, err := recipes.List(ctx)
		if err != nil {
			return nil, err
		}
		quick := selectMaxMinutes(rs, 30)
		evs := []*agentv1.AskEvent{
			thinkingEvent("Filtering by cook time…"),
			thinkingEvent("Keeping 30 minutes or less…"),
			toolCallEvent("search_recipes", "max_minutes=30"),
		}
		evs = append(evs, textChunks(fmt.Sprintf(
			"%d recipes come in at 30 minutes or less.", len(quick)))...)
		evs = append(evs, navigateEvent("/recipes", map[string]string{"maxMinutes": "30"}))
		evs = append(evs, recipeRefs(quick)...)
		return append(evs, doneEvent()), nil

	case "ingredient":
		rs, err := recipes.List(ctx)
		if err != nil {
			return nil, err
		}
		match := selectByIngredient(rs, ing)
		evs := []*agentv1.AskEvent{
			thinkingEvent("Searching recipes with " + ing + "…"),
			toolCallEvent("search_recipes", "ingredient="+ing),
		}
		evs = append(evs, textChunks("Here are the recipes with "+ing+".")...)
		evs = append(evs, navigateEvent("/recipes", map[string]string{"ingredient": ing}))
		evs = append(evs, recipeRefs(match)...)
		return append(evs, doneEvent()), nil

	default:
		return []*agentv1.AskEvent{
			thinkingEvent("Hmm, let me think…"),
			textEvent("I can help you find recipes and meals — try \"what are my favorites\", \"what can I cook tonight\", \"something quick\", or an ingredient like chicken."),
			doneEvent(),
		}, nil
	}
}

func recipeRefs(rs []recipe.Recipe) []*agentv1.AskEvent {
	if len(rs) > 3 {
		rs = rs[:3]
	}
	out := make([]*agentv1.AskEvent, 0, len(rs))
	for _, r := range rs {
		out = append(out, referenceEvent("recipe", r.ID, r.Name, recipeSubtitle(r)))
	}
	return out
}

// textChunks splits a reply into a few TextDelta events to mimic token streaming.
func textChunks(s string) []*agentv1.AskEvent {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	const n = 3
	size := (len(words) + n - 1) / n
	var out []*agentv1.AskEvent
	for i := 0; i < len(words); i += size {
		end := i + size
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		if end < len(words) {
			chunk += " "
		}
		out = append(out, textEvent(chunk))
	}
	return out
}

func textEvent(s string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Text{Text: &agentv1.TextDelta{Text: s}}}
}

func thinkingEvent(s string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Thinking{Thinking: &agentv1.Thinking{Text: s}}}
}

func toolCallEvent(name, detail string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_ToolCall{ToolCall: &agentv1.ToolCall{Name: name, Detail: detail}}}
}

func navigateEvent(to string, search map[string]string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Navigate{Navigate: &agentv1.Navigate{To: to, Search: search}}}
}

func referenceEvent(kind, id, title, subtitle string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Reference{Reference: &agentv1.Reference{
		Kind: kind, Id: id, Title: title, Subtitle: subtitle,
	}}}
}

func doneEvent() *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Done{Done: &agentv1.Done{}}}
}
