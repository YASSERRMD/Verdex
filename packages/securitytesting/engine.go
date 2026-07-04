package securitytesting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Engine is the security-testing orchestrator: it composes a Harness
// (the fixed collection of registered Scenarios), the active
// ScopeDocument, and tenant- and permission-scoped Finding/RunRecord
// persistence into one set of operations, recording every suite run,
// finding open/transition, and remediation verification via AuditSink.
// Engine mirrors packages/compliance.Engine's and
// packages/accessgovernance.Engine's shape closely: authenticate, check
// tenant match, check permission, mutate, audit regardless of outcome.
type Engine struct {
	harness *Harness
	scope   ScopeDocument
	runs    RunRecordRepository
	finds   FindingRepository
	audit   *AuditSink
	clock   func() time.Time
}

// NewEngine builds an Engine from its dependencies. harness, runs, and
// finds must be non-nil (ErrNilStore). scope must satisfy Validate
// (ErrInvalidScope) -- callers with no bespoke engagement scope should
// pass DefaultScopeDocument(). audit may be nil (a nil audit sink means
// suite runs and finding operations simply skip audit recording --
// useful for lightweight unit tests of the decision logic itself,
// though production callers should always supply one).
func NewEngine(harness *Harness, scope ScopeDocument, runs RunRecordRepository, finds FindingRepository, audit *AuditSink) (*Engine, error) {
	if harness == nil || runs == nil || finds == nil {
		return nil, ErrNilStore
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	return &Engine{
		harness: harness,
		scope:   scope,
		runs:    runs,
		finds:   finds,
		audit:   audit,
		clock:   time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// Scope returns the ScopeDocument this Engine was built with.
func (e *Engine) Scope() ScopeDocument { return e.scope }

// RunSuite runs every registered Scenario in category against
// tenantID, requiring the caller to hold managePermission and belong
// to tenantID, persists every resulting RunRecord, and records the
// suite run via AuditSink regardless of outcome. It returns every
// RunRecord produced -- callers that want only the failures should
// filter with FailedRecords. RunSuite does not itself open Findings
// for failed records; call OpenFindingsFromRun for that, so a caller
// that only wants a CI-style pass/fail signal (AllPassed) is not forced
// to also create persistent Finding rows.
func (e *Engine) RunSuite(ctx context.Context, tenantID uuid.UUID, category Category) ([]RunRecord, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}

	records := e.harness.RunCategory(ctx, category, tenantID, user.ID)

	var persistErr error
	for i := range records {
		if err := e.runs.Create(ctx, tenantID, &records[i]); err != nil {
			persistErr = wrapf("RunSuite", err)
			break
		}
	}

	if e.audit != nil {
		_, _ = e.audit.RecordSuiteRun(ctx, tenantID, user.ID, records, persistErr)
	}
	if persistErr != nil {
		return nil, persistErr
	}
	return records, nil
}

// OpenFindingsFromRun opens one Finding per failed RunRecord in
// records (records with any other Outcome are skipped -- a passing or
// errored run never becomes a Finding), requiring managePermission.
// severity is applied to every opened Finding uniformly; callers that
// need per-scenario severity should call OpenFinding directly per
// record instead. Returns every Finding opened, in the same order as
// the failed records within the input slice.
func (e *Engine) OpenFindingsFromRun(ctx context.Context, tenantID uuid.UUID, records []RunRecord, severity Severity) ([]Finding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	if !severity.IsValid() {
		return nil, wrapf("OpenFindingsFromRun", ErrInvalidFinding)
	}

	out := make([]Finding, 0)
	for _, rr := range records {
		if rr.Result.Outcome != OutcomeFailed {
			continue
		}
		f, err := e.openFindingLocked(ctx, tenantID, user.ID, rr, severity)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// OpenFinding opens a single Finding from run, requiring
// managePermission. Unlike OpenFindingsFromRun, this does not require
// run.Result.Outcome == OutcomeFailed -- a caller may deliberately open
// a Finding to track an OutcomeError run that needs investigation, for
// instance.
func (e *Engine) OpenFinding(ctx context.Context, tenantID uuid.UUID, run RunRecord, severity Severity) (Finding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return Finding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Finding{}, err
	}
	return e.openFindingLocked(ctx, tenantID, user.ID, run, severity)
}

func (e *Engine) openFindingLocked(ctx context.Context, tenantID uuid.UUID, actorID uuid.UUID, run RunRecord, severity Severity) (Finding, error) {
	now := e.now()
	f := &Finding{
		ID:             uuid.New(),
		TenantID:       tenantID,
		Title:          fmt.Sprintf("%s: %s", run.ScenarioName, run.Result.Outcome),
		Category:       run.ScenarioCategory,
		Severity:       severity,
		SourceScenario: run.ScenarioName,
		SourceRunID:    run.ID,
		Detail:         run.Result.Detail,
		Status:         FindingOpen,
		OpenedBy:       actorID,
		OpenedAt:       now,
		UpdatedAt:      now,
	}
	if err := f.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingOpen(ctx, tenantID, actorID, *f, err)
		}
		return Finding{}, err
	}
	if err := e.finds.Create(ctx, tenantID, f); err != nil {
		wrapped := wrapf("OpenFinding", err)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingOpen(ctx, tenantID, actorID, *f, wrapped)
		}
		return Finding{}, wrapped
	}
	if e.audit != nil {
		_, _ = e.audit.RecordFindingOpen(ctx, tenantID, actorID, *f, nil)
	}
	return *f, nil
}

// TransitionFinding moves findingID from its current Status to to,
// requiring managePermission and a permitted transition per
// CanTransitionFinding. Moving to FindingVerifiedFixed directly is
// rejected with ErrIllegalStatusTransition regardless of
// CanTransitionFinding's map -- that state is reachable only through
// VerifyRemediation, which requires an actual passing re-run, never
// through this general-purpose transition method. Moving to
// FindingRiskAccepted requires a non-blank justification.
func (e *Engine) TransitionFinding(ctx context.Context, tenantID, findingID uuid.UUID, to FindingStatus, riskJustification string) (Finding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return Finding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Finding{}, err
	}

	f, err := e.finds.Get(ctx, tenantID, findingID)
	if err != nil {
		return Finding{}, err
	}

	transitionErr := e.validateTransition(f.Status, to)
	if transitionErr == nil && to == FindingRiskAccepted && riskJustification == "" {
		transitionErr = wrapf("TransitionFinding", ErrInvalidFinding)
	}
	if transitionErr != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTransition(ctx, tenantID, user.ID, findingID, f.Status, to, transitionErr)
		}
		return Finding{}, transitionErr
	}

	from := f.Status
	f.Status = to
	f.UpdatedAt = e.now()
	if to == FindingRiskAccepted {
		f.RiskAcceptedJustification = riskJustification
	}
	if err := e.finds.Update(ctx, tenantID, f); err != nil {
		wrapped := wrapf("TransitionFinding", err)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingTransition(ctx, tenantID, user.ID, findingID, from, to, wrapped)
		}
		return Finding{}, wrapped
	}
	if e.audit != nil {
		_, _ = e.audit.RecordFindingTransition(ctx, tenantID, user.ID, findingID, from, to, nil)
	}
	return *f, nil
}

// validateTransition applies the shared transition rules every
// TransitionFinding call must satisfy, independent of audit recording.
func (e *Engine) validateTransition(from, to FindingStatus) error {
	if to == FindingVerifiedFixed {
		return wrapf("TransitionFinding", ErrIllegalStatusTransition)
	}
	if !CanTransitionFinding(from, to) {
		return wrapf("TransitionFinding", ErrIllegalStatusTransition)
	}
	return nil
}

// VerifyRemediation is task 8's remediation-verification re-run: it
// re-runs findingID's SourceScenario through the Harness and, only if
// that re-run's Result.Outcome is OutcomePassed, transitions the
// Finding to FindingVerifiedFixed. If the re-run still reports
// OutcomeFailed (the vulnerability still reproduces), the Finding's
// Status is left completely unchanged and VerifyRemediation returns
// ErrRemediationNotVerified -- a caller cannot accidentally mark a
// still-broken Finding as fixed just by calling this method, since the
// only way Status ever becomes FindingVerifiedFixed is through this
// method observing an actual passing run. Requires managePermission,
// and the Finding must currently be FindingRemediationPending (the
// only status CanTransitionFinding permits moving to
// FindingVerifiedFixed from).
func (e *Engine) VerifyRemediation(ctx context.Context, tenantID, findingID uuid.UUID) (Finding, RunRecord, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return Finding{}, RunRecord{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Finding{}, RunRecord{}, err
	}

	f, err := e.finds.Get(ctx, tenantID, findingID)
	if err != nil {
		return Finding{}, RunRecord{}, err
	}
	if f.Status != FindingRemediationPending {
		err := wrapf("VerifyRemediation", ErrIllegalStatusTransition)
		if e.audit != nil {
			_, _ = e.audit.RecordRemediationVerify(ctx, tenantID, user.ID, findingID, false, err)
		}
		return Finding{}, RunRecord{}, err
	}

	rerun, ok := e.harness.RunOne(ctx, f.SourceScenario, tenantID, user.ID)
	if !ok {
		err := wrapf("VerifyRemediation", ErrInvalidScenario)
		if e.audit != nil {
			_, _ = e.audit.RecordRemediationVerify(ctx, tenantID, user.ID, findingID, false, err)
		}
		return Finding{}, RunRecord{}, err
	}
	if err := e.runs.Create(ctx, tenantID, &rerun); err != nil {
		wrapped := wrapf("VerifyRemediation", err)
		if e.audit != nil {
			_, _ = e.audit.RecordRemediationVerify(ctx, tenantID, user.ID, findingID, false, wrapped)
		}
		return Finding{}, RunRecord{}, wrapped
	}

	if rerun.Result.Outcome != OutcomePassed {
		// Deliberately does NOT mutate f.Status: a Finding stuck in
		// FindingRemediationPending after a failed re-run is exactly
		// correct state -- it is still open, just with one more
		// documented (and now persisted) re-run attempt on record.
		notVerified := ErrRemediationNotVerified
		if e.audit != nil {
			_, _ = e.audit.RecordRemediationVerify(ctx, tenantID, user.ID, findingID, false, notVerified)
		}
		return *f, rerun, notVerified
	}

	from := f.Status
	f.Status = FindingVerifiedFixed
	f.UpdatedAt = e.now()
	if err := e.finds.Update(ctx, tenantID, f); err != nil {
		wrapped := wrapf("VerifyRemediation", err)
		if e.audit != nil {
			_, _ = e.audit.RecordRemediationVerify(ctx, tenantID, user.ID, findingID, false, wrapped)
		}
		return Finding{}, rerun, wrapped
	}
	if e.audit != nil {
		_, _ = e.audit.RecordRemediationVerify(ctx, tenantID, user.ID, findingID, true, nil)
		_, _ = e.audit.RecordFindingTransition(ctx, tenantID, user.ID, findingID, from, FindingVerifiedFixed, nil)
	}
	return *f, rerun, nil
}

// ListFindings returns every Finding for tenantID, requiring
// viewPermission.
func (e *Engine) ListFindings(ctx context.Context, tenantID uuid.UUID) ([]Finding, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	findings, err := e.finds.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListFindings", err)
	}
	return findings, nil
}

// ListOpenFindings returns every non-terminal Finding for tenantID
// (FindingOpen, FindingTriaged, FindingRemediationPending), requiring
// viewPermission -- the outstanding-work list a dashboard wants.
func (e *Engine) ListOpenFindings(ctx context.Context, tenantID uuid.UUID) ([]Finding, error) {
	all, err := e.ListFindings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]Finding, 0)
	for _, f := range all {
		if f.IsOpenLike() {
			out = append(out, f)
		}
	}
	return out, nil
}

// ListRuns returns every RunRecord for tenantID, requiring
// viewPermission.
func (e *Engine) ListRuns(ctx context.Context, tenantID uuid.UUID) ([]RunRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	runs, err := e.runs.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListRuns", err)
	}
	return runs, nil
}
