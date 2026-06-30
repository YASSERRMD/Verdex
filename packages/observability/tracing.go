package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Span represents a single unit of traced work. Callers obtain one
// from Tracer.Start and must call End exactly once, typically via
// defer.
type Span interface {
	// SetAttributes attaches key-value metadata to the span.
	SetAttributes(attrs ...Attribute)
	// RecordError records err as an exception event on the span. If
	// err is nil, RecordError is a no-op.
	RecordError(err error)
	// End completes the span. Subsequent calls are safe no-ops, matching
	// the underlying OpenTelemetry SDK's behavior.
	End()
}

// Attribute is a single span attribute key-value pair.
type Attribute struct {
	Key   string
	Value any
}

// String returns a string-valued Attribute.
func String(key, value string) Attribute { return Attribute{Key: key, Value: value} }

// Int returns an int-valued Attribute.
func Int(key string, value int) Attribute { return Attribute{Key: key, Value: value} }

// Bool returns a bool-valued Attribute.
func Bool(key string, value bool) Attribute { return Attribute{Key: key, Value: value} }

// Float64 returns a float64-valued Attribute.
func Float64(key string, value float64) Attribute { return Attribute{Key: key, Value: value} }

func toOtelKeyValues(attrs []Attribute) []attribute.KeyValue {
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		switch v := a.Value.(type) {
		case string:
			kvs = append(kvs, attribute.String(a.Key, v))
		case int:
			kvs = append(kvs, attribute.Int(a.Key, v))
		case bool:
			kvs = append(kvs, attribute.Bool(a.Key, v))
		case float64:
			kvs = append(kvs, attribute.Float64(a.Key, v))
		default:
			kvs = append(kvs, attribute.String(a.Key, fmtAttr(v)))
		}
	}
	return kvs
}

func fmtAttr(v any) string {
	return stringify(v)
}

// Tracer starts spans. It abstracts OpenTelemetry's tracer behind a
// minimal interface so application code does not depend on the
// OpenTelemetry API directly.
type Tracer interface {
	// Start begins a new Span named name as a child of any span
	// already present in ctx, and returns a derived context carrying
	// the new span plus the Span itself. The caller must call
	// Span.End() (typically via defer) when the unit of work
	// completes.
	Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
}

// otelTracer implements Tracer over an OpenTelemetry trace.Tracer.
type otelTracer struct {
	tracer trace.Tracer
}

// otelSpan implements Span over an OpenTelemetry trace.Span.
type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) SetAttributes(attrs ...Attribute) {
	s.span.SetAttributes(toOtelKeyValues(attrs)...)
}

func (s *otelSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

func (s *otelSpan) End() {
	s.span.End()
}

func (t *otelTracer) Start(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
	spanCtx, span := t.tracer.Start(ctx, name, trace.WithAttributes(toOtelKeyValues(attrs)...))
	return spanCtx, &otelSpan{span: span}
}

// TracerProvider wraps an OpenTelemetry SDK TracerProvider, providing
// Tracer instances and a Shutdown hook to flush/release resources.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
}

// NewInMemoryTracerProvider returns a TracerProvider wired with an
// in-memory span recorder instead of a real OTLP exporter, so spans
// can be created and inspected in tests (and during local development)
// without standing up a collector. Use Recorder() to retrieve the
// spans recorded so far.
func NewInMemoryTracerProvider() (*TracerProvider, *SpanRecorder) {
	recorder := newSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(recorder.exporter),
	)
	return &TracerProvider{provider: provider}, recorder
}

// NewNoopTracerProvider returns a TracerProvider that creates spans
// (so call sites behave identically) but discards them rather than
// exporting or recording them anywhere. Use this as the default
// TracerProvider for services that have not yet wired a real exporter.
func NewNoopTracerProvider() *TracerProvider {
	provider := sdktrace.NewTracerProvider()
	return &TracerProvider{provider: provider}
}

// Tracer returns a Tracer scoped to the given instrumentation name
// (typically the service or package name).
func (p *TracerProvider) Tracer(name string) Tracer {
	return &otelTracer{tracer: p.provider.Tracer(name)}
}

// Shutdown flushes and releases resources held by the underlying
// OpenTelemetry SDK TracerProvider. Call it once during graceful
// shutdown.
func (p *TracerProvider) Shutdown(ctx context.Context) error {
	return p.provider.Shutdown(ctx)
}
