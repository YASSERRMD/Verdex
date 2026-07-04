package garelease

import (
	"time"

	"github.com/google/uuid"
)

// Engine is the release-readiness orchestrator: it aggregates
// CheckReadiness's six dimensions, freezes/cuts ReleaseCandidate and
// Release records, and runs the guardrail/audit verification harnesses
// and PostReleaseChecklist -- all of it composing with, never
// duplicating, the upstream packages named in doc.go.
type Engine struct {
	candidates ReleaseCandidateRepository
	releases   ReleaseRepository
	audit      *AuditSink

	// auditStore and representativeTenantID back checkAuditCompleteness
	// (called from CheckReadiness). Both may be left unset (nil /
	// uuid.Nil): CheckReadiness still runs and reports
	// DimensionAuditCompleteness as CheckFailed in that case (fail
	// closed) rather than silently skipping the dimension, and a caller
	// that only wants to call VerifyAuditTrail directly (supplying its
	// own store/tenantID per call) never needs to set these at all.
	auditStore             AuditTrailStore
	representativeTenantID uuid.UUID

	clock func() time.Time
}

// NewEngine builds an Engine from its dependencies. candidates and
// releases must be non-nil (ErrNilStore); audit may be nil (a nil audit
// sink means candidate/release operations simply skip audit recording
// -- useful for lightweight unit tests of the decision logic itself,
// though production callers should always supply one).
func NewEngine(candidates ReleaseCandidateRepository, releases ReleaseRepository, audit *AuditSink) (*Engine, error) {
	if candidates == nil || releases == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		candidates: candidates,
		releases:   releases,
		audit:      audit,
		clock:      time.Now,
	}, nil
}

// WithAuditTrailStore configures e's AuditTrailStore and a
// representative tenant ID for CheckReadiness's
// DimensionAuditCompleteness dimension, returning e for chaining. Both
// are optional: an Engine with neither set still evaluates every other
// dimension and reports DimensionAuditCompleteness as CheckFailed (fail
// closed, per the doc comment on Engine.auditStore).
func (e *Engine) WithAuditTrailStore(store AuditTrailStore, representativeTenantID uuid.UUID) *Engine {
	e.auditStore = store
	e.representativeTenantID = representativeTenantID
	return e
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}
