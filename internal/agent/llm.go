package agent

import (
	"context"
	"fmt"
	"strings"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
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
	Text       string
	ToolCalls  []llmToolCall
	ResponseID string
}

// llmClient is the narrow seam over the OpenAI Responses API. Slice 6b wraps the
// real implementation with Sigil; tests use a stub.
type llmClient interface {
	Respond(ctx context.Context, prev string, toolOutputs []llmToolOutput, userText string) (llmTurn, error)
}

type llmAgent struct {
	recipes   *recipe.Repo
	meals     *meal.Repo
	client    llmClient
	maxRounds int
}

func newLLMAgent(client llmClient, recipes *recipe.Repo, meals *meal.Repo) *llmAgent {
	return &llmAgent{recipes: recipes, meals: meals, client: client, maxRounds: 5}
}

// Run drives the tool-calling loop, emitting AskEvents as each round completes.
func (a *llmAgent) Run(ctx context.Context, userText string, emit func(*agentv1.AskEvent) error) error {
	prev := ""
	var outputs []llmToolOutput
	first := true

	for round := 0; round < a.maxRounds; round++ {
		text := ""
		if first {
			text = userText
		}
		turn, err := a.client.Respond(ctx, prev, outputs, text)
		if err != nil {
			return err
		}
		first = false
		prev = turn.ResponseID
		outputs = nil

		// No tool calls -> final answer.
		if len(turn.ToolCalls) == 0 {
			if err := emitText(turn.Text, emit); err != nil {
				return err
			}
			return emit(doneEvent())
		}

		// Execute each tool call, streaming its events and collecting outputs.
		for _, tc := range turn.ToolCalls {
			if err := emit(thinkingEvent(thinkingFor(tc))); err != nil {
				return err
			}
			if err := emit(toolCallEvent(tc.Name, detailFor(tc))); err != nil {
				return err
			}
			res, err := executeTool(ctx, a.recipes, a.meals, tc.Name, tc.Arguments)
			if err != nil {
				res = toolResult{Summary: fmt.Sprintf(`{"error":%q}`, err.Error())}
			}
			for _, ev := range res.Events {
				if err := emit(ev); err != nil {
					return err
				}
			}
			outputs = append(outputs, llmToolOutput{CallID: tc.CallID, Output: res.Summary})
		}
	}

	// Hit the round cap: ask once more for a final summary, best-effort.
	turn, err := a.client.Respond(ctx, prev, outputs, "")
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
