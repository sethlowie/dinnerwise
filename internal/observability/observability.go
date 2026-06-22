// Package observability wires OpenTelemetry (traces + metrics over OTLP) and a
// Grafana Sigil client for AI Observability. It is additive: with no OTLP
// endpoint configured, Init returns a no-op so the app runs unchanged.
package observability

import (
	"context"

	"github.com/grafana/sigil-sdk/go/sigil"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/sethlowie/dinnerwise/internal/config"
)

const instrumentationScope = "github.com/sethlowie/dinnerwise/internal/agent"

// Providers holds the active OTel tracer and optional Sigil client.
type Providers struct {
	Tracer trace.Tracer
	Sigil  *sigil.Client
}

// Init bootstraps OpenTelemetry and (optionally) a Grafana Sigil client.
// When cfg.HasObservability() is false it returns a no-op Tracer, nil Sigil,
// and a no-op shutdown — no exporters are created and no env vars are required.
func Init(ctx context.Context, cfg config.Config) (*Providers, func(context.Context) error, error) {
	if !cfg.HasObservability() {
		return &Providers{Tracer: noop.NewTracerProvider().Tracer(instrumentationScope)},
			func(context.Context) error { return nil },
			nil
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)))
	if err != nil {
		return nil, nil, err
	}

	spanExp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(spanExp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)

	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader), sdkmetric.WithResource(res))
	otel.SetMeterProvider(mp)

	// Sigil: instrumentation-only. Generation export off (no proprietary ingest).
	scfg := sigil.DefaultConfig()
	scfg.GenerationExport.Protocol = sigil.GenerationExportProtocolNone
	sclient := sigil.NewClient(scfg)

	shutdown := func(ctx context.Context) error {
		_ = sclient.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return tp.Shutdown(ctx)
	}
	return &Providers{Tracer: tp.Tracer(instrumentationScope), Sigil: sclient}, shutdown, nil
}
