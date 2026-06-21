// Package agent holds the AgentService: it streams typed events — assistant
// text plus meta (thinking, tool calls), a navigate action, and reference
// cards — in response to user text. When an OPENAI_API_KEY is present the
// real LLM agent is used; otherwise a scripted fallback runs instead.
package agent

import (
	"context"
	"time"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
	"github.com/sethlowie/dinnerwise/internal/config"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// Service implements agentv1connect.AgentServiceHandler. When agent is non-nil
// it drives the LLM path; otherwise it uses the scripted fallback with delay.
type Service struct {
	recipes *recipe.Repo
	meals   *meal.Repo
	delay   time.Duration
	agent   *llmAgent
}

// NewService returns a handler backed by the real OpenAI agent when cfg has a
// key, and the scripted fallback (with a lifelike 60 ms delay) otherwise.
func NewService(cfg config.Config, recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler {
	if cfg.HasOpenAI() {
		client := newOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
		return &Service{recipes: recipes, meals: meals, agent: newLLMAgent(client, recipes, meals)}
	}
	return &Service{recipes: recipes, meals: meals, delay: 60 * time.Millisecond}
}

// NewServiceWithDelay returns a handler with an explicit delay (use 0 in tests).
func NewServiceWithDelay(recipes *recipe.Repo, meals *meal.Repo, d time.Duration) agentv1connect.AgentServiceHandler {
	return &Service{recipes: recipes, meals: meals, delay: d}
}

func (s *Service) Ask(
	ctx context.Context,
	req *connect.Request[agentv1.AskRequest],
	stream *connect.ServerStream[agentv1.AskEvent],
) error {
	if s.agent != nil {
		emit := func(ev *agentv1.AskEvent) error { return stream.Send(ev) }
		err := s.agent.Run(ctx, req.Msg.GetText(), emit)
		if err != nil {
			// Best-effort graceful close: a short apology then done.
			_ = stream.Send(textEvent("Sorry — I hit a problem reaching the model. Try again?"))
			_ = stream.Send(doneEvent())
			return nil
		}
		return nil
	}
	// scripted path (unchanged)
	events, err := respond(ctx, s.recipes, s.meals, req.Msg.GetText())
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, ev := range events {
		if err := stream.Send(ev); err != nil {
			return err
		}
		if s.delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.delay):
			}
		}
	}
	return nil
}
