package agent

import (
	"context"
	"math"
	"sync"
	"testing"
)

func TestCostUSDKnownModel(t *testing.T) {
	// gpt-5.4 priced in the table; 1M in + 1M out should equal in+out price.
	got := costUSD("gpt-5.4", 1_000_000, 1_000_000)
	want := priceTable["gpt-5.4"].inPerM + priceTable["gpt-5.4"].outPerM
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("costUSD = %v, want %v", got, want)
	}
}

func TestCostUSDUnknownModelUsesDefault(t *testing.T) {
	got := costUSD("totally-unknown", 1_000_000, 0)
	if math.Abs(got-defaultPrice.inPerM) > 1e-9 {
		t.Fatalf("costUSD = %v, want default %v", got, defaultPrice.inPerM)
	}
}

func TestCostUSDZero(t *testing.T) {
	if got := costUSD("gpt-5.4", 0, 0); got != 0 {
		t.Fatalf("costUSD zero tokens = %v, want 0", got)
	}
}

func TestRecordCostConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); recordCost(context.Background(), "gpt-5.4", 100, 50) }()
	}
	wg.Wait()
}
