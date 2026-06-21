package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
)

func TestAskStreamsScriptedEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(agentv1connect.NewAgentServiceHandler(NewServiceWithDelay(0)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := agentv1connect.NewAgentServiceClient(http.DefaultClient, srv.URL)
	stream, err := client.Ask(context.Background(),
		connect.NewRequest(&agentv1.AskRequest{Text: "chicken"}))
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	var sawNavigate, sawDone bool
	for stream.Receive() {
		switch stream.Msg().Event.(type) {
		case *agentv1.AskEvent_Navigate:
			sawNavigate = true
		case *agentv1.AskEvent_Done:
			sawDone = true
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if !sawNavigate {
		t.Fatal("expected a navigate event over the stream")
	}
	if !sawDone {
		t.Fatal("expected a done event over the stream")
	}
}
