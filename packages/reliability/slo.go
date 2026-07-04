package reliability

import (
	"sort"
	"time"
)

// SLOKind names what dimension an SLO measures.
type SLOKind string

const (
	// SLOKindSuccessRate targets a minimum fraction of observed calls
	// that must succeed over the rolling window (Target is a value in
	// [0,1], e.g. 0.995 for "99.5% of calls succeed").
	SLOKindSuccessRate SLOKind = "success_rate"

	// SLOKindLatency targets a maximum P95 latency over the rolling
	// window (Target is a time.Duration expressed as float64
	// nanoseconds, mirroring packages/perf.DimensionVerdict's uniform
	// float64 convention for cross-dimension comparability).
	SLOKindLatency SLOKind = "latency"
)

// SLO defines a service-level objective: a named target (a minimum
// success rate, or a maximum P95 latency) evaluated over a rolling
// time window, mirroring the target/observed-and-compare shape
// packages/perf.Budget/Evaluate already established for benchmark
// targets (Phase 091) -- applied here to live, production traffic
// rather than a CI benchmark run. See doc.go's "perf vs reliability"
// discussion for why these are deliberately parallel, not merged,
// concepts.
type SLO struct {
	// Name identifies this SLO (e.g. "ingestion-availability",
	// "hybrid-retrieval-latency").
	Name string

	// Kind names which dimension this SLO targets.
	Kind SLOKind

	// Target is the objective value: for SLOKindSuccessRate, a
	// fraction in [0,1]; for SLOKindLatency, a maximum acceptable P95
	// latency.
	Target float64

	// Window is the rolling period observations are evaluated over
	// (e.g. 30 * 24h for a rolling 30-day window).
	Window time.Duration
}

// Validate checks s for structural well-formedness.
func (s SLO) Validate() error {
	if s.Name == "" {
		return ErrInvalidSLO
	}
	if s.Window <= 0 {
		return ErrInvalidSLO
	}
	switch s.Kind {
	case SLOKindSuccessRate:
		if s.Target < 0 || s.Target > 1 {
			return ErrInvalidSLO
		}
	case SLOKindLatency:
		if s.Target <= 0 {
			return ErrInvalidSLO
		}
	default:
		return ErrInvalidSLO
	}
	return nil
}

// Observation is a single recorded call outcome contributing to an
// SLO's rolling-window evaluation.
type Observation struct {
	// Success reports whether the call succeeded. Always meaningful,
	// even for a latency SLO (a failed call's latency, if any, is
	// excluded from latency percentile computation -- see
	// EvaluateSLO).
	Success bool

	// Latency is the observed call latency. Ignored for
	// SLOKindSuccessRate SLOs.
	Latency time.Duration

	// At is when this observation was recorded, used to filter to
	// SLO.Window's rolling period.
	At time.Time
}

// SLOStatus reports the outcome of evaluating an SLO against a set of
// Observations: the observed value alongside the target, and whether
// the objective is currently being met.
type SLOStatus struct {
	// SLO is the objective evaluated.
	SLO SLO

	// Observed is the computed value for SLO.Kind: an observed success
	// rate in [0,1], or an observed P95 latency in nanoseconds.
	Observed float64

	// SampleCount is how many Observations fell within the rolling
	// window and were used to compute Observed.
	SampleCount int

	// Met reports whether Observed satisfies SLO.Target.
	Met bool
}

// EvaluateSLO filters observations to those within SLO.Window of now,
// computes the relevant observed value for slo.Kind, and reports
// whether the objective is currently met. An empty (post-filter)
// observation set is reported as Met=true with SampleCount=0 -- an SLO
// with no traffic yet has not been violated, mirroring how
// packages/perf.CompareRuns treats an empty historical baseline as "no
// regression" rather than a hard failure.
//
// Returns a wrapped ErrInvalidSLO if slo fails structural validation.
func EvaluateSLO(slo SLO, observations []Observation, now time.Time) (SLOStatus, error) {
	if err := slo.Validate(); err != nil {
		return SLOStatus{}, wrapf("EvaluateSLO", err)
	}

	windowStart := now.Add(-slo.Window)
	var inWindow []Observation
	for _, o := range observations {
		if o.At.After(windowStart) && !o.At.After(now) {
			inWindow = append(inWindow, o)
		}
	}

	if len(inWindow) == 0 {
		return SLOStatus{SLO: slo, Met: true}, nil
	}

	switch slo.Kind {
	case SLOKindSuccessRate:
		successes := 0
		for _, o := range inWindow {
			if o.Success {
				successes++
			}
		}
		rate := float64(successes) / float64(len(inWindow))
		return SLOStatus{
			SLO:         slo,
			Observed:    rate,
			SampleCount: len(inWindow),
			Met:         rate >= slo.Target,
		}, nil

	case SLOKindLatency:
		p95 := p95Latency(inWindow)
		return SLOStatus{
			SLO:         slo,
			Observed:    float64(p95),
			SampleCount: len(inWindow),
			Met:         float64(p95) <= slo.Target,
		}, nil

	default:
		return SLOStatus{}, wrapf("EvaluateSLO", ErrInvalidSLO)
	}
}

// p95Latency computes the 95th-percentile latency across successful
// observations using the same nearest-rank percentile convention
// packages/perf documents for its own latency aggregation (see
// packages/perf/doc.go, LoadTest). Failed calls are excluded: a
// latency SLO measures how fast successful responses are, not how
// quickly a call fails.
func p95Latency(observations []Observation) time.Duration {
	latencies := make([]time.Duration, 0, len(observations))
	for _, o := range observations {
		if o.Success {
			latencies = append(latencies, o.Latency)
		}
	}
	if len(latencies) == 0 {
		return 0
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	// Nearest-rank: ceil(0.95 * n), 1-indexed, clamped to the last
	// element.
	rank := int(0.95*float64(len(latencies)) + 0.9999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(latencies) {
		rank = len(latencies)
	}
	return latencies[rank-1]
}
