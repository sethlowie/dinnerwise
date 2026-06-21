// Package agent holds the (currently scripted, no-LLM) AgentService: it streams
// typed events — assistant text plus meta (thinking, tool calls) and a
// navigate action — in response to user text.
package agent

import (
	"context"
	"time"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
)

// Service implements agentv1connect.AgentServiceHandler by streaming the events
// produced by script(). delay paces the stream to simulate token streaming.
type Service struct {
	delay time.Duration
}

// NewService returns a handler with a lifelike streaming delay.
func NewService() agentv1connect.AgentServiceHandler {
	return &Service{delay: 60 * time.Millisecond}
}

// NewServiceWithDelay returns a handler with an explicit delay (use 0 in tests).
func NewServiceWithDelay(d time.Duration) agentv1connect.AgentServiceHandler {
	return &Service{delay: d}
}

func (s *Service) Ask(
	ctx context.Context,
	req *connect.Request[agentv1.AskRequest],
	stream *connect.ServerStream[agentv1.AskEvent],
) error {
	for _, ev := range script(req.Msg.GetText()) {
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
