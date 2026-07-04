// Package alerting's metrics.go registers a small, named catalogue of
// business-relevant counters and gauges through
// packages/observability.Registry (task 1). This file never
// constructs a Prometheus registry, a Counter, or a Gauge type of its
// own -- every metric named below is obtained from the caller's
// existing observability.Registry, so a service that already exposes
// "/metrics" via that Registry picks up these business metrics
// automatically, with no second exposition endpoint.
package alerting

import "github.com/YASSERRMD/verdex/packages/observability"

// Metric names this catalogue registers. Kept as named constants
// (rather than only inline strings) so AlertRule.Condition.MetricName
// and DashboardDefinition panels can reference them without risking a
// typo'd literal silently failing to match at evaluation time.
const (
	// MetricCasesIngestedTotal counts cases that have completed
	// ingestion (packages/ingestion's pipeline finishing for a case),
	// partitioned by outcome ("success"/"failure").
	MetricCasesIngestedTotal = "verdex_cases_ingested_total"

	// MetricOpinionsSignedOffTotal counts opinions that have completed
	// the sign-off workflow (packages/signoff), partitioned by
	// disposition ("approved"/"rejected").
	MetricOpinionsSignedOffTotal = "verdex_opinions_signed_off_total"

	// MetricSARRequestsPending is a gauge of currently pending
	// data-subject-access requests (packages/privacy.SubjectAccessRequest
	// not yet resolved), partitioned by tenant-agnostic default label
	// only -- callers that want a per-tenant breakdown pass a tenant
	// label value when calling Inc/Dec/Set.
	MetricSARRequestsPending = "verdex_sar_requests_pending"

	// MetricReasoningRunsTotal counts reasoning-pipeline runs
	// completed (packages/reasoningorchestration), partitioned by
	// outcome.
	MetricReasoningRunsTotal = "verdex_reasoning_runs_total"

	// MetricAlertsFiredTotal counts AlertEvents this package itself
	// has raised, partitioned by severity -- a meta-metric letting an
	// operator see alerting volume trends over time (are we getting
	// noisier?) alongside the business metrics above.
	MetricAlertsFiredTotal = "verdex_alerts_fired_total"

	// MetricSyntheticCheckLatencySeconds is a histogram of
	// SyntheticCheck probe latencies (synthetic.go, task 8),
	// partitioned by check name and outcome.
	MetricSyntheticCheckLatencySeconds = "verdex_synthetic_check_latency_seconds"
)

// Catalogue holds the real Counter/Gauge/Histogram handles this
// package's business metrics resolve to, obtained from a single
// observability.Registry via RegisterBusinessMetrics. Callers hold one
// Catalogue per process (typically alongside their own
// observability.Registry) and pass it to whichever code path observes
// each business event.
type Catalogue struct {
	// CasesIngestedTotal is MetricCasesIngestedTotal, labeled by
	// "outcome".
	CasesIngestedTotal observability.Counter

	// OpinionsSignedOffTotal is MetricOpinionsSignedOffTotal, labeled
	// by "disposition".
	OpinionsSignedOffTotal observability.Counter

	// SARRequestsPending is MetricSARRequestsPending, labeled by
	// "tenant".
	SARRequestsPending observability.Gauge

	// ReasoningRunsTotal is MetricReasoningRunsTotal, labeled by
	// "outcome".
	ReasoningRunsTotal observability.Counter

	// AlertsFiredTotal is MetricAlertsFiredTotal, labeled by
	// "severity".
	AlertsFiredTotal observability.Counter

	// SyntheticCheckLatencySeconds is
	// MetricSyntheticCheckLatencySeconds, labeled by "check" and
	// "outcome".
	SyntheticCheckLatencySeconds observability.Histogram
}

// RegisterBusinessMetrics registers this package's named metric
// catalogue through registry and returns the resulting Catalogue.
// Returns ErrNilStore if registry is nil. Safe to call once per
// process per Registry -- calling it twice against the same Registry
// with the same metric names will panic via the underlying Prometheus
// client (MustRegister), exactly as calling
// observability.Registry.Counter twice with the same name would;
// callers should build exactly one Catalogue per Registry, mirroring
// how a service builds exactly one observability.Registry per
// process.
func RegisterBusinessMetrics(registry observability.Registry) (*Catalogue, error) {
	if registry == nil {
		return nil, ErrNilStore
	}
	return &Catalogue{
		CasesIngestedTotal: registry.Counter(
			MetricCasesIngestedTotal,
			"Total number of cases that have completed ingestion, by outcome.",
			"outcome",
		),
		OpinionsSignedOffTotal: registry.Counter(
			MetricOpinionsSignedOffTotal,
			"Total number of opinions that have completed sign-off, by disposition.",
			"disposition",
		),
		SARRequestsPending: registry.Gauge(
			MetricSARRequestsPending,
			"Current count of pending subject-access requests, by tenant.",
			"tenant",
		),
		ReasoningRunsTotal: registry.Counter(
			MetricReasoningRunsTotal,
			"Total number of reasoning-pipeline runs completed, by outcome.",
			"outcome",
		),
		AlertsFiredTotal: registry.Counter(
			MetricAlertsFiredTotal,
			"Total number of alerting.AlertEvents raised, by severity.",
			"severity",
		),
		SyntheticCheckLatencySeconds: registry.Histogram(
			MetricSyntheticCheckLatencySeconds,
			"Observed latency of synthetic monitoring probes, by check and outcome.",
			nil,
			"check", "outcome",
		),
	}, nil
}

// RecordAlertFired increments AlertsFiredTotal for severity. A small
// convenience so Engine.Evaluate-adjacent call sites can observe every
// firing without each one re-deriving the label value; c may be nil,
// in which case this is a no-op (a caller that has not wired up a
// Catalogue yet should not have to guard every call site).
func (c *Catalogue) RecordAlertFired(severity Severity) {
	if c == nil || c.AlertsFiredTotal == nil {
		return
	}
	c.AlertsFiredTotal.Inc(string(severity))
}
