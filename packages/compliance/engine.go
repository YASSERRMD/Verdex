package compliance

import (
	"time"
)

// Engine is the compliance-mapping orchestrator: it composes the
// Control catalogue, per-tenant ControlEvidence, Profile selection,
// and gap-analysis/dashboard reporting into one set of tenant- and
// permission-scoped operations, recording every control registration,
// evidence addition, and profile change via AuditSink. Engine mirrors
// packages/privacy.Engine's shape closely: authenticate, check tenant
// match, check permission, mutate, audit regardless of outcome.
type Engine struct {
	controls ControlRepository
	evidence EvidenceRepository
	profiles ProfileRepository
	audit    *AuditSink
	clock    func() time.Time
}

// NewEngine builds an Engine from its dependencies. controls, evidence,
// and profiles must be non-nil (ErrNilStore); audit may be nil (a nil
// audit sink means control/evidence/profile operations simply skip
// audit recording -- useful for lightweight unit tests of the decision
// logic itself, though production callers should always supply one).
func NewEngine(controls ControlRepository, evidence EvidenceRepository, profiles ProfileRepository, audit *AuditSink) (*Engine, error) {
	if controls == nil || evidence == nil || profiles == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		controls: controls,
		evidence: evidence,
		profiles: profiles,
		audit:    audit,
		clock:    time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}
