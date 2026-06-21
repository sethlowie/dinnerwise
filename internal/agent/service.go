// Package agent holds the (currently scripted, no-LLM) AgentService: it streams
// typed events — assistant text plus meta (thinking, tool calls), a navigate
// action, and reference cards — in response to user text. Scenarios query the
// recipe/meal repos, prefiguring the tools a real LLM agent would call.
package agent

import (
	"context"
	"time"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// Service implements agentv1connect.AgentServiceHandler by streaming the events
// produced by respond(). delay paces the stream to simulate token streaming.
type Service struct {
	recipes *recipe.Repo
	meals   *meal.Repo
	delay   time.Duration
}

// NewService returns a handler with a lifelike streaming delay.
func NewService(recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler {
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
