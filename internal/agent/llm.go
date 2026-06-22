package agent

import (
	"context"
	"fmt"
	"strings"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const systemPrompt = `You are Sous, a concise kitchen copilot for a home cook.
You have tools to search the user's recipes and cooked-meal log, and to navigate
the app's UI. Use the tools to look things up — never invent recipes or meals
that the tools did not return. After finding what the user asked for, call the
navigate tool to take them to the right view, then give a one or two sentence
spoken summary.

Navigation targets:
- /recipes with search {"ingredient":"<x>"} | {"pantry":"1"} | {"maxMinutes":"30"}
- /meals with search {"sort":"rating","fav":"1"} | {"sort":"recent"}
- a specific recipe or meal's detail page: /recipes/<recipe_id> or
  /meals/<meal_id>. When the user asks about one specific dish (e.g. "show me
  the tomato pasta recipe" or "how do I make it"), search first to get its id,
  then navigate to its detail page.

Keep replies short and warm.`

type llmToolCall struct {
	CallID    string
	Name      string
	Arguments string
}

type llmToolOutput struct {
	CallID string
	Output string
}

type llmTurn struct {
	Text      string
	ToolCalls []llmToolCall
}

// llmItem is one entry in the running conversation. Exactly one field is set: a
// user message, an assistant tool call to echo back, or a tool call's output.
// We accumulate these and resend the whole list each round because the org runs
// the Responses API in stateless mode (Zero Data Retention forbids
// previous_response_id chaining).
type llmItem struct {
	UserText      string
	AssistantText string
	ToolCall      *llmToolCall
	ToolOutput    *llmToolOutput
}

// llmClient is the narrow seam over the OpenAI Responses API. Slice 6b wraps the
// real implementation with Sigil; tests use a stub. The full conversation is
// passed each call (stateless) — see llmItem.
type llmClient interface {
	Respond(ctx context.Context, items []llmItem) (llmTurn, error)
}

type llmAgent struct {
	recipes   *recipe.Repo
	meals     *meal.Repo
	client    llmClient
	maxRounds int
	tracer    trace.Tracer
}

func newLLMAgent(client llmClient, recipes *recipe.Repo, meals *meal.Repo, tracer trace.Tracer) *llmAgent {
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("agent")
	}
	return &llmAgent{recipes: recipes, meals: meals, client: client, maxRounds: 5, tracer: tracer}
}

// Run drives the tool-calling loop, emitting AskEvents as each round completes.
// history holds prior conversation turns (oldest first); they are prepended so
// the model has context. The conversation is accumulated in items and resent in
// full each round (stateless; no previous_response_id).
func (a *llmAgent) Run(ctx context.Context, history []llmItem, userText string, emit func(*agentv1.AskEvent) error) error {
	items := append([]llmItem{}, history...)
	items = append(items, llmItem{UserText: userText})

	for round := 0; round < a.maxRounds; round++ {
		turn, err := a.client.Respond(ctx, items)
		if err != nil {
			return err
		}

		// No tool calls -> final answer.
		if len(turn.ToolCalls) == 0 {
			if err := emitText(turn.Text, emit); err != nil {
				return err
			}
			return emit(doneEvent())
		}

		// Execute each tool call, streaming its events. Echo the assistant calls
		// then their outputs into the running conversation for the next round.
		var outputs []llmItem
		for _, tc := range turn.ToolCalls {
			if err := emit(thinkingEvent(thinkingFor(tc))); err != nil {
				return err
			}
			if err := emit(toolCallEvent(tc.Name, detailFor(tc))); err != nil {
				return err
			}
			res, err := func() (toolResult, error) {
				ctx, span := a.tracer.Start(ctx, "agent.tool",
					trace.WithAttributes(attribute.String("gen_ai.tool.name", tc.Name)))
				defer span.End()
				return executeTool(ctx, a.recipes, a.meals, tc.Name, tc.Arguments)
			}()
			if err != nil {
				res = toolResult{Summary: fmt.Sprintf(`{"error":%q}`, err.Error())}
			}
			for _, ev := range res.Events {
				if err := emit(ev); err != nil {
					return err
				}
			}
			call := tc
			items = append(items, llmItem{ToolCall: &call})
			outputs = append(outputs, llmItem{ToolOutput: &llmToolOutput{CallID: tc.CallID, Output: res.Summary}})
		}
		items = append(items, outputs...)
	}

	// Hit the round cap: ask once more for a final summary, best-effort.
	turn, err := a.client.Respond(ctx, items)
	if err == nil {
		if err := emitText(turn.Text, emit); err != nil {
			return err
		}
	}
	return emit(doneEvent())
}

func emitText(s string, emit func(*agentv1.AskEvent) error) error {
	for _, ev := range textChunks(s) {
		if err := emit(ev); err != nil {
			return err
		}
	}
	return nil
}

func thinkingFor(tc llmToolCall) string {
	switch tc.Name {
	case toolSearchRecipes:
		return "Searching your recipes…"
	case toolSearchMeals:
		return "Checking your meals log…"
	case toolNavigate:
		return "Opening the right view…"
	}
	return "Working…"
}

func detailFor(tc llmToolCall) string {
	a := strings.TrimSpace(tc.Arguments)
	if a == "" || a == "{}" {
		return tc.Name
	}
	return a
}
