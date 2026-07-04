package e2e

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

// staticCaseVersionReader is a minimal, package-local
// signoff.CaseVersionReader fake reporting a fixed version for every
// case, mirroring packages/signoff/helpers_test.go's own
// fakeCaseVersionReader convention -- this package deliberately does
// not import packages/caselifecycle's full Repository just to satisfy
// this narrow interface (see doc/e2e-suite.md's "reused, not
// duplicated" section: packages/signoff's own doc/signoff-workflow.md
// already documents the real caselifecycle.Repository adapter a
// production caller wires in; this suite's synthetic cases have no
// real caselifecycle.Case to read a live MetadataVersion from).
type staticCaseVersionReader struct {
	mu       sync.Mutex
	versions map[uuid.UUID]int
}

func newStaticCaseVersionReader() *staticCaseVersionReader {
	return &staticCaseVersionReader{versions: make(map[uuid.UUID]int)}
}

func (r *staticCaseVersionReader) set(caseID uuid.UUID, version int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.versions[caseID] = version
}

func (r *staticCaseVersionReader) CaseVersion(_ context.Context, _, caseID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.versions[caseID]; ok {
		return v, nil
	}
	return 1, nil
}

var _ signoff.CaseVersionReader = (*staticCaseVersionReader)(nil)

// NewSignoffEnforcementScenario builds task 6's sign-off enforcement
// scenario: it proves, with real state assertions against the actual
// packages/signoff.Service and packages/guardrail.CanFinalize gate
// (not a simulated boolean), that:
//
//  1. A case with no recorded sign-off cannot finalize
//     (guardrail.CanFinalize reports false, SignoffPending).
//  2. Recording an Approve decision through signoff.Service --
//     requiring the exact AcknowledgementConfirmation phrase, a
//     matching CaseVersion, and an authenticated actor holding
//     identity.PermSignOff -- flips that same gate to reporting true,
//     SignoffApproved.
//  3. Attempting to Approve WITHOUT the exact acknowledgement phrase
//     is rejected (ErrAcknowledgementRequired), proving the
//     enforcement is not bypassable by an empty or wrong string.
func NewSignoffEnforcementScenario() (Scenario, error) {
	return NewScenarioFunc("civil/signoff-enforcement", category.CodeCivil, runSignoffEnforcement)
}

func runSignoffEnforcement(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	tenantID := uuid.New()
	caseID := uuid.New()

	repo := signoff.NewInMemoryRepository()
	reader := newStaticCaseVersionReader()
	reader.set(caseID, 1)

	svc, err := signoff.NewService(repo, reader, nil)
	if err != nil {
		return ScenarioResult{}, wrapf("runSignoffEnforcement", err)
	}

	gate, err := signoff.NewGate(repo, tenantID)
	if err != nil {
		return ScenarioResult{}, wrapf("runSignoffEnforcement", err)
	}

	caseIDStr := caseID.String()

	// Step 1: before any decision is recorded, finalization must be
	// blocked -- the fail-closed default this whole gate exists to
	// guarantee.
	approvedBefore, statusErr := guardrail.CanFinalize(ctx, caseIDStr, gate)
	if approvedBefore {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: CanFinalize reported true before any sign-off decision was recorded", ErrSignoffNotEnforced),
			CaseID:     caseIDStr,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}
	if statusErr == nil {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: CanFinalize returned a nil error before any sign-off decision was recorded (expected ErrSignoffNotApproved)", ErrSignoffNotEnforced),
			CaseID:     caseIDStr,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	// Step 2: attempting to Approve without the exact acknowledgement
	// phrase must fail -- proving the requirement is not satisfiable by
	// an empty or approximate string.
	judgeCtx, _ := authenticatedContext(tenantID, identity.RoleJudge)
	_, badAckErr := svc.Approve(judgeCtx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: "I approve", // deliberately wrong
	})
	if badAckErr == nil {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: Approve succeeded with an incorrect acknowledgement string", ErrSignoffNotEnforced),
			CaseID:     caseIDStr,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	// Step 3: recording a real, correctly-acknowledged Approve decision
	// must flip the gate.
	if _, err := svc.Approve(judgeCtx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Notes:           "e2e suite: reviewed and approved for finalization",
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		return ScenarioResult{}, wrapf("runSignoffEnforcement: Approve", err)
	}

	approvedAfter, err := guardrail.CanFinalize(ctx, caseIDStr, gate)
	if err != nil {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: CanFinalize still errored after a real Approve decision was recorded: %v", ErrSignoffNotEnforced, err),
			CaseID:     caseIDStr,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}
	if !approvedAfter {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: CanFinalize reported false after a real Approve decision was recorded", ErrSignoffNotEnforced),
			CaseID:     caseIDStr,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	return ScenarioResult{
		Outcome:           OutcomePassed,
		Detail:            "finalization was blocked absent sign-off, rejected an incorrect acknowledgement, and succeeded once a real Approve decision was recorded",
		CaseID:            caseIDStr,
		GuardrailApproved: approvedAfter,
		StartedAt:         startedAt,
		FinishedAt:        time.Now().UTC(),
	}, nil
}
