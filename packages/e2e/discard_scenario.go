package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/intake"
)

// discardCheckPayload is the synthetic PDF-sniffing exhibit body this
// scenario submits through the real intake pipeline, mirroring
// packages/ingestion/orchestrator.go's own intakeArtifactHeader
// convention (a minimal payload that both declares and sniffs as
// application/pdf).
var discardCheckPayload = []byte("%PDF-1.4 verdex e2e discard-guarantee synthetic artifact")

// NewDiscardGuaranteeScenario builds task 5's discard-guarantee
// verification scenario: it drives a synthetic exhibit through the
// REAL packages/intake.IntakeService.Ingest pipeline (validate ->
// quota -> sniff -> buffer -> hash -> scan -> ready -> scheduled
// discard) with a short TTL, then asserts the service's own
// post-processing state -- IntakeResult.Status() ==
// intake.StatusDiscarded and DiscardedAt() populated -- rather than a
// boolean flag this scenario invented itself. This mirrors
// packages/intake/discard_test.go's own
// TestIngest_BufferDiscardedAfterTTL assertion shape exactly, called
// through the same service a real deployment uses.
func NewDiscardGuaranteeScenario() (Scenario, error) {
	return NewScenarioFunc("civil/discard-guarantee", category.CodeCivil, runDiscardGuarantee)
}

func runDiscardGuarantee(ctx context.Context) (ScenarioResult, error) {
	startedAt := time.Now().UTC()

	svc := intake.NewIntakeService(intake.NoOpVirusScanHook{}, nil, intake.NoOpAuditSink{}, 0)

	const ttl = 150 * time.Millisecond
	req := intake.IntakeRequest{
		TenantID:   uuid.New(),
		UploaderID: uuid.New(),
		Filename:   "e2e-discard-guarantee-exhibit.pdf",
		MIMEType:   "application/pdf",
		SizeBytes:  int64(len(discardCheckPayload)),
		TTL:        ttl,
	}

	result, err := svc.Ingest(ctx, req, strings.NewReader(string(discardCheckPayload)))
	if err != nil {
		return ScenarioResult{}, wrapf("runDiscardGuarantee: Ingest", err)
	}
	if result.Status() != intake.StatusReady {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("expected status ready immediately after Ingest, got %q", result.Status()),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}
	if result.ProvisionHash == "" {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     "expected a non-empty provenance hash after Ingest",
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	// Wait past the TTL plus a safety margin -- the real service's own
	// background goroutine (see intake.IntakeService.ingest's step 8)
	// is what performs the discard, not this scenario simulating one.
	select {
	case <-time.After(ttl + 400*time.Millisecond):
	case <-ctx.Done():
		return ScenarioResult{}, wrapf("runDiscardGuarantee", ctx.Err())
	}

	if result.Status() != intake.StatusDiscarded {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: expected status discarded after TTL, got %q", ErrDiscardVerificationFailed, result.Status()),
			CaseID:     result.IntakeID.String(),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}
	if result.DiscardedAt() == nil {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("%v: DiscardedAt was not populated after TTL elapsed", ErrDiscardVerificationFailed),
			CaseID:     result.IntakeID.String(),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	// DiscardAll must be a safe, idempotent no-op once the background
	// TTL goroutine has already discarded the buffer -- re-verifying the
	// real service's own double-discard safety, not merely assuming it.
	if err := svc.DiscardAll(ctx, result.IntakeID); err != nil {
		return ScenarioResult{
			Outcome:    OutcomeFailed,
			Detail:     fmt.Sprintf("DiscardAll after TTL-triggered discard returned an error: %v", err),
			CaseID:     result.IntakeID.String(),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}, nil
	}

	return ScenarioResult{
		Outcome:    OutcomePassed,
		Detail:     "intake service's real post-processing state confirms the binary artifact is discarded and no longer reachable",
		CaseID:     result.IntakeID.String(),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	}, nil
}
