package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
)

func TestTracer_RecordsSpan(t *testing.T) {
	provider, recorder := NewInMemoryTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tracer := provider.Tracer("test")
	_, span := tracer.Start(context.Background(), "do-work", String("key", "value"))
	span.End()

	spans := recorder.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 recorded span, got %d", len(spans))
	}
	if spans[0].Name() != "do-work" {
		t.Errorf("span name = %q, want %q", spans[0].Name(), "do-work")
	}

	attrs := spans[0].Attributes()
	found := false
	for _, a := range attrs {
		if string(a.Key) == "key" && a.Value.AsString() == "value" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected attribute key=value in span attributes: %v", attrs)
	}
}

func TestTracer_NestedSpansShareTrace(t *testing.T) {
	provider, recorder := NewInMemoryTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tracer := provider.Tracer("test")
	ctx, parent := tracer.Start(context.Background(), "parent")
	_, child := tracer.Start(ctx, "child")
	child.End()
	parent.End()

	spans := recorder.Spans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].SpanContext().TraceID() != spans[1].SpanContext().TraceID() {
		t.Error("expected parent and child spans to share a trace ID")
	}
}

func TestSpan_RecordError(t *testing.T) {
	provider, recorder := NewInMemoryTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tracer := provider.Tracer("test")
	_, span := tracer.Start(context.Background(), "failing-op")
	span.RecordError(errors.New("boom"))
	span.End()

	spans := recorder.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", spans[0].Status().Code)
	}
	events := spans[0].Events()
	if len(events) == 0 {
		t.Fatal("expected an exception event to be recorded")
	}
}

func TestSpan_RecordErrorNilIsNoop(t *testing.T) {
	provider, recorder := NewInMemoryTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tracer := provider.Tracer("test")
	_, span := tracer.Start(context.Background(), "ok-op")
	span.RecordError(nil)
	span.End()

	spans := recorder.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code == codes.Error {
		t.Error("expected status to remain unset for nil error")
	}
}

func TestNoopTracerProvider_CreatesUsableSpans(t *testing.T) {
	provider := NewNoopTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tracer := provider.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "noop-op", Int("n", 1), Bool("ok", true), Float64("ratio", 0.5))
	span.SetAttributes(String("extra", "field"))
	span.End()

	if ctx == nil {
		t.Error("expected a non-nil context from Start")
	}
}
