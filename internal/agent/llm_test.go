package agent

import (
	"context"
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// stubClient returns a fixed script of turns, ignoring inputs.
type stubClient struct {
	turns []llmTurn
	i     int
}

func (s *stubClient) Respond(_ context.Context, _ []llmItem) (llmTurn, error) {
	t := s.turns[s.i]
	s.i++
	return t, nil
}

func collect(t *testing.T, a *llmAgent, text string) []*agentv1.AskEvent {
	t.Helper()
	var got []*agentv1.AskEvent
	if err := a.Run(context.Background(), text, func(e *agentv1.AskEvent) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func kinds(evs []*agentv1.AskEvent) []string {
	var k []string
	for _, e := range evs {
		switch e.Event.(type) {
		case *agentv1.AskEvent_Thinking:
			k = append(k, "thinking")
		case *agentv1.AskEvent_ToolCall:
			k = append(k, "tool_call")
		case *agentv1.AskEvent_Reference:
			k = append(k, "reference")
		case *agentv1.AskEvent_Navigate:
			k = append(k, "navigate")
		case *agentv1.AskEvent_Text:
			k = append(k, "text")
		case *agentv1.AskEvent_Done:
			k = append(k, "done")
		}
	}
	return k
}

func TestRunToolThenText(t *testing.T) {
	recipes, meals := seededRepos(t)
	client := &stubClient{turns: []llmTurn{
		{ToolCalls: []llmToolCall{{CallID: "c1", Name: toolSearchRecipes, Arguments: `{"ingredient":"chicken"}`}}},
		{Text: "Here are chicken recipes."},
	}}
	a := newLLMAgent(client, recipes, meals, nil)
	a.maxRounds = 5
	got := kinds(collect(t, a, "chicken please"))
	// thinking + tool_call precede the references; text then done at the end.
	if got[0] != "thinking" || got[1] != "tool_call" {
		t.Fatalf("want thinking,tool_call first; got %v", got)
	}
	if got[len(got)-1] != "done" {
		t.Fatalf("want done last; got %v", got)
	}
	var sawRef, sawText bool
	for _, k := range got {
		sawRef = sawRef || k == "reference"
		sawText = sawText || k == "text"
	}
	if !sawRef || !sawText {
		t.Fatalf("want reference and text; got %v", got)
	}
}

func TestRunEmitsSpans(t *testing.T) {
	recipes, meals := seededRepos(t)
	client := &stubClient{turns: []llmTurn{
		{ToolCalls: []llmToolCall{{CallID: "c1", Name: toolSearchRecipes, Arguments: `{"ingredient":"chicken"}`}}},
		{Text: "done"},
	}}
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	a := newLLMAgent(client, recipes, meals, tp.Tracer("test"))

	ctx, parent := tp.Tracer("test").Start(context.Background(), "agent.ask")
	if err := a.Run(ctx, "chicken", func(*agentv1.AskEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	parent.End()

	var sawTool bool
	for _, s := range sr.Ended() {
		if s.Name() == "agent.tool" {
			sawTool = true
		}
	}
	if !sawTool {
		t.Fatal("expected an agent.tool span")
	}
}

func TestRunMaxRoundsCap(t *testing.T) {
	recipes, meals := seededRepos(t)
	// Always calls a tool -> would loop forever without the cap.
	always := llmTurn{ToolCalls: []llmToolCall{{CallID: "c", Name: toolSearchRecipes, Arguments: `{}`}}}
	client := &stubClient{turns: []llmTurn{always, always, always, always, always, always, always}}
	a := newLLMAgent(client, recipes, meals, nil)
	a.maxRounds = 5
	got := kinds(collect(t, a, "loop"))
	if got[len(got)-1] != "done" {
		t.Fatalf("want done last even at cap; got %v", got)
	}
	// Loop runs at most maxRounds (5) times, plus one final summary call = 6 total.
	if client.i > 6 {
		t.Fatalf("client called %d times, want <= 6", client.i)
	}
}
