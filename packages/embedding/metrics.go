package embedding

// EmbeddingMetrics aggregates throughput counters for a single [EmbeddingService]
// operation.  All fields are non-negative integers.
type EmbeddingMetrics struct {
	// TotalEmbedded is the number of text strings processed (cache hits +
	// cache misses) in this operation.
	TotalEmbedded int64

	// CacheHits is the number of texts resolved from the cache without
	// calling the upstream provider.
	CacheHits int64

	// CacheMisses is the number of texts not found in cache that required a
	// provider call.
	CacheMisses int64

	// BatchCalls is the number of individual provider.Embed invocations made
	// (each may cover up to batchSize texts).
	BatchCalls int64

	// Errors is the number of operations that terminated with a non-nil
	// error.
	Errors int64
}

// MetricsSink receives [EmbeddingMetrics] snapshots after each [EmbeddingService]
// operation completes.  Implementations must not block; if slow processing is
// needed, buffer internally.
//
// Implementations MUST be safe for concurrent use.
type MetricsSink interface {
	Record(m EmbeddingMetrics)
}

// NoOpMetricsSink is a [MetricsSink] that discards all metrics.  It is used
// as the default when no sink is configured.
type NoOpMetricsSink struct{}

// Record implements [MetricsSink] by doing nothing.
func (NoOpMetricsSink) Record(_ EmbeddingMetrics) {}

// AccumulatingMetricsSink collects metrics across multiple operations and
// exposes a running total.  It is primarily intended for tests and dashboards.
type AccumulatingMetricsSink struct {
	totals EmbeddingMetrics
	mu     interface{ Lock(); Unlock() }
}

// NewAccumulatingMetricsSink returns an [AccumulatingMetricsSink] ready to
// receive metrics.
func NewAccumulatingMetricsSink() *AccumulatingMetricsSink {
	return &AccumulatingMetricsSink{
		mu: newMu(),
	}
}

// Record implements [MetricsSink].
func (a *AccumulatingMetricsSink) Record(m EmbeddingMetrics) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.totals.TotalEmbedded += m.TotalEmbedded
	a.totals.CacheHits += m.CacheHits
	a.totals.CacheMisses += m.CacheMisses
	a.totals.BatchCalls += m.BatchCalls
	a.totals.Errors += m.Errors
}

// Totals returns a copy of the accumulated metrics.
func (a *AccumulatingMetricsSink) Totals() EmbeddingMetrics {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totals
}

// Reset zeroes all accumulated counters.
func (a *AccumulatingMetricsSink) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.totals = EmbeddingMetrics{}
}
