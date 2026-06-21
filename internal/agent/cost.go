package agent

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// modelPrice is USD per 1,000,000 tokens. Approximate, for a local demo cost
// figure — not real billing. Update as pricing/models change.
type modelPrice struct {
	inPerM  float64
	outPerM float64
}

var priceTable = map[string]modelPrice{
	"gpt-5.4":    {inPerM: 1.25, outPerM: 10.0},
	"gpt-5-nano": {inPerM: 0.05, outPerM: 0.40},
}

// defaultPrice is used for models not in the table.
var defaultPrice = modelPrice{inPerM: 1.0, outPerM: 3.0}

func priceFor(model string) modelPrice {
	if p, ok := priceTable[model]; ok {
		return p
	}
	return defaultPrice
}

// costUSD computes an approximate dollar cost for a generation.
func costUSD(model string, inTokens, outTokens int64) float64 {
	p := priceFor(model)
	return float64(inTokens)/1e6*p.inPerM + float64(outTokens)/1e6*p.outPerM
}

var costCounter metric.Float64Counter

func costInstrument() metric.Float64Counter {
	if costCounter != nil {
		return costCounter
	}
	m := otel.GetMeterProvider().Meter("github.com/sethlowie/dinnerwise/internal/agent")
	c, err := m.Float64Counter("gen_ai.client.cost.usd",
		metric.WithDescription("Approximate USD cost of generations"),
		metric.WithUnit("{USD}"))
	if err != nil {
		return nil
	}
	costCounter = c
	return c
}

// recordCost adds an approximate dollar cost for one generation.
func recordCost(ctx context.Context, model string, inTokens, outTokens int64) {
	c := costInstrument()
	if c == nil {
		return
	}
	c.Add(ctx, costUSD(model, inTokens, outTokens),
		metric.WithAttributes(attribute.String("gen_ai.request.model", model)))
}
