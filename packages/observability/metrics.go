package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Counter is a monotonically increasing metric, optionally
// label-partitioned (e.g. requests served, errors encountered).
type Counter interface {
	// Inc increments the counter by 1 for the given label values, in
	// the order the labels were declared when the counter was
	// created.
	Inc(labelValues ...string)
	// Add increments the counter by delta (which must be >= 0) for the
	// given label values.
	Add(delta float64, labelValues ...string)
}

// Gauge is a metric that can move up or down (e.g. in-flight requests,
// queue depth), optionally label-partitioned.
type Gauge interface {
	// Set sets the gauge to v for the given label values.
	Set(v float64, labelValues ...string)
	// Inc increments the gauge by 1 for the given label values.
	Inc(labelValues ...string)
	// Dec decrements the gauge by 1 for the given label values.
	Dec(labelValues ...string)
}

// Histogram observes a distribution of values (e.g. request latency),
// optionally label-partitioned.
type Histogram interface {
	// Observe records a single observation for the given label values.
	Observe(v float64, labelValues ...string)
}

// Registry creates and registers metrics. It is a thin abstraction
// over the underlying metrics backend (Prometheus's client_golang in
// this implementation) so application code depends on this interface
// rather than importing a specific metrics client.
type Registry interface {
	// Counter returns a Counter named name (Prometheus naming
	// conventions apply, e.g. "verdex_requests_total"), partitioned by
	// labels. Calling Counter twice with the same name and label set
	// returns equivalent counters backed by the same underlying
	// series.
	Counter(name, help string, labels ...string) Counter
	// Gauge returns a Gauge named name, partitioned by labels.
	Gauge(name, help string, labels ...string) Gauge
	// Histogram returns a Histogram named name, partitioned by labels,
	// using buckets as the observation bucket boundaries. A nil
	// buckets slice uses Prometheus's default buckets.
	Histogram(name, help string, buckets []float64, labels ...string) Histogram
	// Handler returns an http.Handler that exposes every metric
	// registered through this Registry in Prometheus exposition
	// format, suitable for mounting at "/metrics".
	Handler() http.Handler
}

// promRegistry implements Registry over a dedicated
// *prometheus.Registry (not the global default registry, so multiple
// Registry instances - e.g. one per test - never collide).
type promRegistry struct {
	registry *prometheus.Registry
}

// NewRegistry returns a Registry backed by a fresh, isolated
// Prometheus registry.
func NewRegistry() Registry {
	return &promRegistry{registry: prometheus.NewRegistry()}
}

type promCounter struct {
	vec *prometheus.CounterVec
}

func (c *promCounter) Inc(labelValues ...string) {
	c.vec.WithLabelValues(labelValues...).Inc()
}

func (c *promCounter) Add(delta float64, labelValues ...string) {
	c.vec.WithLabelValues(labelValues...).Add(delta)
}

func (r *promRegistry) Counter(name, help string, labels ...string) Counter {
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labels)
	r.registry.MustRegister(vec)
	return &promCounter{vec: vec}
}

type promGauge struct {
	vec *prometheus.GaugeVec
}

func (g *promGauge) Set(v float64, labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Set(v)
}

func (g *promGauge) Inc(labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Inc()
}

func (g *promGauge) Dec(labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Dec()
}

func (r *promRegistry) Gauge(name, help string, labels ...string) Gauge {
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labels)
	r.registry.MustRegister(vec)
	return &promGauge{vec: vec}
}

type promHistogram struct {
	vec *prometheus.HistogramVec
}

func (h *promHistogram) Observe(v float64, labelValues ...string) {
	h.vec.WithLabelValues(labelValues...).Observe(v)
}

func (r *promRegistry) Histogram(name, help string, buckets []float64, labels ...string) Histogram {
	if buckets == nil {
		buckets = prometheus.DefBuckets
	}
	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: buckets,
	}, labels)
	r.registry.MustRegister(vec)
	return &promHistogram{vec: vec}
}

func (r *promRegistry) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}
