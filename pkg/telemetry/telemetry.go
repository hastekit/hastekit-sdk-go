package telemetry

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func NewLangFuseExporter(endpoint string, username string, password string, secure bool) (trace.SpanExporter, error) {
	endpointWithProto := strings.Replace(endpoint, "http://", "", 1)
	authKey := fmt.Sprintf("%s:%s", username, password)

	opts := []otlptracehttp.Option{
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(authKey))),
		}),
		otlptracehttp.WithEndpoint(endpointWithProto),
		otlptracehttp.WithURLPath("/api/public/otel/v1/traces"),
	}
	if secure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	return otlptracehttp.New(context.Background(), opts...)
}

func newResource() *resource.Resource {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "agent-server"
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion("0.1.0"),
	)
}

// NewProvider creates new telemetry provider, and sets it as a default open telemetry trace provider.
func NewProvider(exp trace.SpanExporter) func() {
	f, err := os.Create("traces.txt")
	if err != nil {
		slog.Error("Unable to create traces.txt", slog.Any("error", err))
		return func() {}
	}

	// Wrap the deployment-wide exporter in a routing exporter that fans
	// spans out per organisation (default pipeline and/or the org's own
	// external OTLP collector). Until SetOrgConfigResolver is called the
	// router is a pass-through to the default exporter. The enrich
	// processor stamps org_id and session.id from baggage onto every span
	// so whole traces — not just root spans — are routable and
	// session-groupable.
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.ParentBased(trace.AlwaysSample())),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(exp)),
		trace.WithResource(newResource()),
	)

	// Surface exporter/processor errors (e.g. a rejected external OTLP
	// export) through slog instead of OTel's default stderr logger, which
	// is easy to miss in structured logs.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("otel error", slog.Any("error", err))
	}))

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return func() {
		if err := f.Close(); err != nil {
			slog.Error("Unable to close traces file", slog.Any("error", err))
		}

		if err := tp.Shutdown(context.Background()); err != nil {
			slog.Error("unable to shutdown trace provider", slog.Any("error", err))
		}
	}
}
