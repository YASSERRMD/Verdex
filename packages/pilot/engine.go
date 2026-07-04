package pilot

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Engine is the pilot-deployment-and-feedback-loop orchestrator: it
// composes a tenant's PilotDeployments, PilotCases, FeedbackEntry
// records, PilotFindings, and RefinementRecords into one set of
// tenant- and permission-scoped operations, recording every
// deployment status change, finding triage, and report capture via
// AuditSink. Engine mirrors packages/compliance.Engine's and
// packages/vulnmanagement.Engine's shape closely: authenticate, check
// tenant match, check permission, mutate, audit regardless of outcome.
type Engine struct {
	deployments DeploymentRepository
	cases       CaseRepository
	feedback    FeedbackRepository
	findings    FindingRepository
	refinements RefinementRepository
	audit       *AuditSink
	clock       func() time.Time
}

// NewEngine builds an Engine from its dependencies. deployments,
// cases, feedback, findings, and refinements must be non-nil
// (ErrNilStore); audit may be nil (a nil audit sink means every
// operation simply skips audit recording -- useful for lightweight
// unit tests of the decision logic itself, though production callers
// should always supply one).
func NewEngine(
	deployments DeploymentRepository,
	cases CaseRepository,
	feedback FeedbackRepository,
	findings FindingRepository,
	refinements RefinementRepository,
	audit *AuditSink,
) (*Engine, error) {
	if deployments == nil || cases == nil || feedback == nil || findings == nil || refinements == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		deployments: deployments,
		cases:       cases,
		feedback:    feedback,
		findings:    findings,
		refinements: refinements,
		audit:       audit,
		clock:       time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// Activity surfaces every pilot-related audit event for tenantID
// matching filter, requiring viewPermission and tenant match,
// delegating to AuditSink.PilotActivity -- which itself queries
// through packages/auditlog.Store's own PermAuditRead-gated Query.
// Returns ErrNilAuditSink if this Engine was constructed with a nil
// audit sink.
func (e *Engine) Activity(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	if e.audit == nil {
		return nil, ErrNilAuditSink
	}
	events, err := e.audit.PilotActivity(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("Activity", err)
	}
	return events, nil
}
