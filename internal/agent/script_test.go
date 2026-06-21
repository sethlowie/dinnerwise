package agent

import (
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

func TestScriptMatchEmitsToolCallAndNavigate(t *testing.T) {
	events := script("what recipes have chicken?")

	var sawToolCall bool
	var nav *agentv1.Navigate
	for _, e := range events {
		switch ev := e.Event.(type) {
		case *agentv1.AskEvent_ToolCall:
			sawToolCall = true
		case *agentv1.AskEvent_Navigate:
			nav = ev.Navigate
		}
	}
	if !sawToolCall {
		t.Fatal("expected a tool_call event")
	}
	if nav == nil {
		t.Fatal("expected a navigate event")
	}
	if nav.GetTo() != "/recipes" {
		t.Fatalf("navigate.to = %q, want /recipes", nav.GetTo())
	}
	if nav.GetSearch()["ingredient"] != "chicken" {
		t.Fatalf("search[ingredient] = %q, want chicken", nav.GetSearch()["ingredient"])
	}
	if _, ok := events[len(events)-1].Event.(*agentv1.AskEvent_Done); !ok {
		t.Fatal("expected last event to be Done")
	}
}

func TestScriptMatchOrdering(t *testing.T) {
	events := script("chicken please")
	if len(events) < 4 {
		t.Fatalf("too few events: %d", len(events))
	}
	if _, ok := events[0].Event.(*agentv1.AskEvent_Thinking); !ok {
		t.Fatal("first event should be Thinking")
	}
	if _, ok := events[1].Event.(*agentv1.AskEvent_ToolCall); !ok {
		t.Fatal("second event should be ToolCall")
	}
	// Navigate must come before Done.
	navIdx, doneIdx := -1, -1
	for i, e := range events {
		switch e.Event.(type) {
		case *agentv1.AskEvent_Navigate:
			navIdx = i
		case *agentv1.AskEvent_Done:
			doneIdx = i
		}
	}
	if navIdx == -1 || doneIdx == -1 || navIdx > doneIdx {
		t.Fatalf("expected Navigate before Done (nav=%d done=%d)", navIdx, doneIdx)
	}
}

func TestScriptNoMatchHasNoNavigate(t *testing.T) {
	events := script("hello there")
	for _, e := range events {
		if _, ok := e.Event.(*agentv1.AskEvent_Navigate); ok {
			t.Fatal("did not expect a navigate event for an unmatched query")
		}
	}
	if _, ok := events[len(events)-1].Event.(*agentv1.AskEvent_Done); !ok {
		t.Fatal("expected last event to be Done")
	}
}
