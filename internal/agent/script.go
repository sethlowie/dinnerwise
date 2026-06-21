package agent

import (
	"strings"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

// knownIngredients are the keywords the scripted (no-LLM) backend recognizes.
// They mirror the seeded recipe fixtures.
var knownIngredients = []string{
	"chicken", "tomato", "tofu", "garlic", "broccoli", "rice", "pasta",
}

// script is the entire hardcoded backend behavior: given user text it returns
// the sequence of events to stream. It is a pure function so it can be unit
// tested without any streaming machinery. The real LLM agent will replace this.
func script(text string) []*agentv1.AskEvent {
	lower := strings.ToLower(text)
	for _, ing := range knownIngredients {
		if strings.Contains(lower, ing) {
			return []*agentv1.AskEvent{
				thinkingEvent("Looking for recipes with " + ing + "…"),
				toolCallEvent("search_recipes", "ingredient="+ing),
				textEvent("Here"),
				textEvent(" are the recipes"),
				textEvent(" with " + ing + "."),
				navigateEvent("/recipes", map[string]string{"ingredient": ing}),
				doneEvent(),
			}
		}
	}
	return []*agentv1.AskEvent{
		thinkingEvent("No specific ingredient mentioned…"),
		textEvent("I can help you find recipes — try asking about an ingredient like chicken."),
		doneEvent(),
	}
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

func doneEvent() *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Done{Done: &agentv1.Done{}}}
}
