package gateway

import (
	"context"
	"maps"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type gatewayContextKey struct{}

func AddContext(ctx context.Context, values map[string]string) context.Context {
	// Check if the context already has the key
	gatewayContext, ok := ctx.Value(gatewayContextKey{}).(map[string]string)
	if ok {
		maps.Copy(gatewayContext, values)
		return context.WithValue(ctx, gatewayContextKey{}, gatewayContext)
	}

	newValues := make(map[string]string, len(values))
	maps.Copy(newValues, values)

	return context.WithValue(ctx, gatewayContextKey{}, values)
}

func addToSpan(ctx context.Context, span trace.Span) {
	if span == nil {
		return
	}

	attributes := []attribute.KeyValue{}

	gatewayContextAny := ctx.Value(gatewayContextKey{})
	gatewayContext, ok := gatewayContextAny.(map[string]string)
	if ok {
		for k, v := range gatewayContext {
			attributes = append(attributes, attribute.String(k, v))
		}

		span.SetAttributes(attributes...)
	}
}
