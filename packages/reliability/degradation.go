package reliability

import "context"

// DegradationMode names a specific, reduced-service fallback behavior a
// Degrader may fall back to when its primary path fails. Deliberately
// an open string type (like packages/compliance.Framework), since new
// degradation strategies are expected to accrue over time without
// requiring a code change to this package.
type DegradationMode string

const (
	// ModeServeStale means the fallback served a cached/stale result
	// instead of a fresh one (e.g. the last known-good hybrid-retrieval
	// result set, an earlier graph-traversal snapshot).
	ModeServeStale DegradationMode = "serve_stale"

	// ModeSkipEnrichment means the fallback completed the primary
	// request but skipped an optional enrichment step (e.g. citation
	// cross-referencing, evidence-weighing enrichment) to keep the
	// core response available.
	ModeSkipEnrichment DegradationMode = "skip_enrichment"

	// ModeReducedScope means the fallback narrowed the operation's
	// scope (e.g. fewer retrieved documents, a shallower traversal
	// depth) rather than failing outright.
	ModeReducedScope DegradationMode = "reduced_scope"

	// ModeStaticDefault means the fallback returned a fixed, safe
	// default value with no dependency on the failed primary path at
	// all.
	ModeStaticDefault DegradationMode = "static_default"
)

// Result[T] wraps a value produced by a Degrader, reporting whether it
// came from the primary path or a degraded fallback, and if degraded,
// which DegradationMode was used.
type Result[T any] struct {
	// Value is the produced value, from either the primary or the
	// fallback function.
	Value T

	// Degraded reports whether Value came from the fallback path
	// rather than the primary.
	Degraded bool

	// Mode names which DegradationMode the fallback used. Zero value
	// (empty string) when Degraded is false.
	Mode DegradationMode
}

// PrimaryFunc[T] is a Degrader's main code path.
type PrimaryFunc[T any] func(ctx context.Context) (T, error)

// FallbackFunc[T] is a Degrader's reduced-service code path, invoked
// only when PrimaryFunc fails. It receives the primary's error so the
// fallback can (optionally) vary its behavior based on why the primary
// failed.
type FallbackFunc[T any] func(ctx context.Context, primaryErr error) (T, error)

// Degrader[T] wraps a primary+fallback function pair implementing
// graceful degradation: try the primary path first; if it fails, run
// the fallback and mark the result as degraded rather than propagating
// the primary's failure to the caller. This composes naturally with
// CircuitBreaker and Retry -- a primary wrapped in Retry/CircuitBreaker
// still degrades to the fallback once those give up.
type Degrader[T any] struct {
	// Mode names which DegradationMode this Degrader's fallback
	// represents, recorded on Result.Mode whenever the fallback runs.
	Mode DegradationMode

	primary  PrimaryFunc[T]
	fallback FallbackFunc[T]
}

// NewDegrader constructs a Degrader with the given primary and
// fallback functions, associating mode with every degraded Result this
// Degrader produces. Both primary and fallback must be non-nil; Run
// returns ErrNilFunc otherwise.
func NewDegrader[T any](mode DegradationMode, primary PrimaryFunc[T], fallback FallbackFunc[T]) *Degrader[T] {
	return &Degrader[T]{Mode: mode, primary: primary, fallback: fallback}
}

// Run executes the primary function. If it succeeds, Run returns its
// value with Degraded=false. If it fails, Run invokes the fallback and
// returns its value with Degraded=true and Mode set -- unless the
// fallback itself also fails, in which case Run returns the fallback's
// error (not the primary's), since that is the more recent, more
// specific failure a caller needs to act on.
func (d *Degrader[T]) Run(ctx context.Context) (Result[T], error) {
	var zero T
	if d.primary == nil || d.fallback == nil {
		return Result[T]{Value: zero}, wrapf("Degrader.Run", ErrNilFunc)
	}

	value, err := d.primary(ctx)
	if err == nil {
		return Result[T]{Value: value}, nil
	}

	fallbackValue, fallbackErr := d.fallback(ctx, err)
	if fallbackErr != nil {
		return Result[T]{Value: zero}, fallbackErr
	}

	return Result[T]{
		Value:    fallbackValue,
		Degraded: true,
		Mode:     d.Mode,
	}, nil
}
