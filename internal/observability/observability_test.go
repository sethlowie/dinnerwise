package observability

import (
	"context"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/config"
)

func TestInitDisabledIsNoop(t *testing.T) {
	p, shutdown, err := Init(context.Background(), config.Config{}) // no endpoint
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Tracer == nil {
		t.Fatal("expected non-nil Providers with a (no-op) Tracer")
	}
	if p.Sigil != nil {
		t.Fatal("expected nil Sigil when disabled")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown should not error: %v", err)
	}
	// Tracer must be usable (no-op) without panicking.
	_, span := p.Tracer.Start(context.Background(), "x")
	span.End()
}
