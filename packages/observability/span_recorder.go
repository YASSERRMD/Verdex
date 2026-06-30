package observability

import (
	"context"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanRecorder captures every span exported by a TracerProvider
// constructed via NewInMemoryTracerProvider, so tests can assert on
// span names, attributes, and status without a live collector.
type SpanRecorder struct {
	exporter *recordingExporter
}

func newSpanRecorder() *SpanRecorder {
	return &SpanRecorder{exporter: &recordingExporter{}}
}

// Spans returns a snapshot of every span recorded so far, in the order
// they were ended.
func (r *SpanRecorder) Spans() []sdktrace.ReadOnlySpan {
	return r.exporter.snapshot()
}

// Reset clears all recorded spans.
func (r *SpanRecorder) Reset() {
	r.exporter.reset()
}

// recordingExporter implements sdktrace.SpanExporter by appending every
// exported span to an in-memory slice.
type recordingExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *recordingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *recordingExporter) Shutdown(_ context.Context) error {
	return nil
}

func (e *recordingExporter) snapshot() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]sdktrace.ReadOnlySpan, len(e.spans))
	copy(out, e.spans)
	return out
}

func (e *recordingExporter) reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = nil
}
