package agent

import (
	"context"
	"os"
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

// TestLiveOpenAI runs one real turn. Skipped unless OPENAI_API_KEY is set and
// DINNERWISE_LIVE=1, so the default suite never calls the network.
func TestLiveOpenAI(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" || os.Getenv("DINNERWISE_LIVE") != "1" {
		t.Skip("set OPENAI_API_KEY and DINNERWISE_LIVE=1 to run the live test")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-nano"
	}
	recipes, meals := seededRepos(t)
	a := newLLMAgent(newOpenAIClient(key, model, nil), recipes, meals, nil)

	var text, done bool
	err := a.Run(context.Background(), nil, "What can I cook tonight?", func(e *agentv1.AskEvent) error {
		switch e.Event.(type) {
		case *agentv1.AskEvent_Text:
			text = true
		case *agentv1.AskEvent_Done:
			done = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !text || !done {
		t.Fatalf("expected text and done; text=%v done=%v", text, done)
	}
}
