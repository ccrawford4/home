package api

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

func traceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

func spanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().SpanID().String()
}
