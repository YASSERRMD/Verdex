package securitytesting

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/intake"
)

// This file is task 6's abuse-case suite: three concrete, real,
// testable abuse cases relevant to this platform -- quota abuse,
// oversized-payload abuse, and replay -- rather than a generic,
// untargeted load-testing scenario.

// ScenarioIntakeDailyQuotaAbuse drives packages/intake's
// InMemoryQuotaChecker (Phase 019) past its configured
// MaxDailyUploadsPerTenant for a single tenant and proves the
// (N+1)-th request in the same rolling day is rejected with
// ErrQuotaExceeded -- a real, named platform quota, not a
// hypothetical one this suite invents.
func ScenarioIntakeDailyQuotaAbuse() Scenario {
	return NewScenarioFunc(
		"abuse-case/intake-daily-quota-abuse",
		CategoryAbuseCase,
		func(ctx context.Context) (Result, error) {
			const dailyLimit = 3
			checker := intake.NewInMemoryQuotaChecker(intake.QuotaConfig{
				MaxDailyUploadsPerTenant: dailyLimit,
			})
			tenantID := uuid.New()
			req := intake.IntakeRequest{TenantID: tenantID, SizeBytes: 1024}

			for i := 0; i < dailyLimit; i++ {
				if err := checker.Check(ctx, tenantID, req); err != nil {
					return Result{
						Outcome: OutcomeFailed,
						Detail:  fmt.Sprintf("Check() call %d/%d (within limit) unexpectedly failed: %v", i+1, dailyLimit, err),
					}, nil
				}
				checker.RecordComplete(tenantID)
			}

			// The (dailyLimit+1)-th upload attempt in the same day must be
			// rejected -- this is the abuse attempt: a tenant (or a
			// compromised credential for that tenant) hammering the
			// intake endpoint past its documented daily allowance.
			err := checker.Check(ctx, tenantID, req)
			if err == nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Check() call %d (over the daily limit of %d) was allowed, want ErrQuotaExceeded", dailyLimit+1, dailyLimit),
				}, nil
			}
			if !errors.Is(err, intake.ErrQuotaExceeded) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Check() over daily limit returned %v, want an error wrapping intake.ErrQuotaExceeded", err),
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: fmt.Sprintf("intake.InMemoryQuotaChecker correctly rejected upload %d after a %d-upload daily limit", dailyLimit+1, dailyLimit)}, nil
		},
	)
}

// ScenarioIntakeConcurrentQuotaAbuse proves
// intake.InMemoryQuotaChecker's MaxConcurrentPerTenant limit rejects a
// burst of concurrent in-flight uploads for one tenant once the
// configured ceiling is reached, and that RecordComplete correctly
// frees a slot for the next request -- the abuse case being a tenant
// opening many uploads simultaneously without ever completing them, to
// exhaust a shared resource.
func ScenarioIntakeConcurrentQuotaAbuse() Scenario {
	return NewScenarioFunc(
		"abuse-case/intake-concurrent-quota-abuse",
		CategoryAbuseCase,
		func(ctx context.Context) (Result, error) {
			const concurrentLimit = 2
			checker := intake.NewInMemoryQuotaChecker(intake.QuotaConfig{
				MaxConcurrentPerTenant: concurrentLimit,
			})
			tenantID := uuid.New()
			req := intake.IntakeRequest{TenantID: tenantID, SizeBytes: 1024}

			for i := 0; i < concurrentLimit; i++ {
				if err := checker.Check(ctx, tenantID, req); err != nil {
					return Result{
						Outcome: OutcomeFailed,
						Detail:  fmt.Sprintf("Check() call %d/%d (within concurrent limit) unexpectedly failed: %v", i+1, concurrentLimit, err),
					}, nil
				}
				// Deliberately no RecordComplete here -- these uploads are
				// left "in flight", exactly the abuse pattern this scenario
				// probes.
			}

			if err := checker.Check(ctx, tenantID, req); !errors.Is(err, intake.ErrQuotaExceeded) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Check() with %d uploads already in flight (limit %d) = %v, want ErrQuotaExceeded", concurrentLimit, concurrentLimit, err),
				}, nil
			}

			// Completing one in-flight upload must free exactly one slot.
			checker.RecordComplete(tenantID)
			if err := checker.Check(ctx, tenantID, req); err != nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Check() after RecordComplete freed a slot still failed: %v", err),
				}, nil
			}

			return Result{Outcome: OutcomePassed, Detail: "intake.InMemoryQuotaChecker correctly caps concurrent in-flight uploads per tenant and RecordComplete frees a slot"}, nil
		},
	)
}

// ScenarioIntakeOversizedPayloadAbuse proves
// intake.ValidateSizeMB rejects a payload declared far larger than the
// configured per-tenant size ceiling -- the abuse case being a
// malicious or misbehaving client declaring (or streaming) a payload
// sized to exhaust temporary storage or downstream OCR/STT processing
// capacity.
func ScenarioIntakeOversizedPayloadAbuse() Scenario {
	return NewScenarioFunc(
		"abuse-case/intake-oversized-payload-abuse",
		CategoryAbuseCase,
		func(_ context.Context) (Result, error) {
			const limitMB = 25
			oversizedBytes := int64(limitMB+100) * 1024 * 1024 // 100MB past the ceiling

			if err := intake.ValidateSizeMB(oversizedBytes, limitMB); err == nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("ValidateSizeMB(%d bytes, limit %dMB) = nil, want a rejection error", oversizedBytes, limitMB),
				}, nil
			}

			// A payload at exactly the ceiling must still be accepted --
			// this suite also proves the check is not overly strict
			// (rejecting legitimate uploads at the documented limit would
			// itself be a functional regression, not a security win).
			atLimitBytes := int64(limitMB) * 1024 * 1024
			if err := intake.ValidateSizeMB(atLimitBytes, limitMB); err != nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("ValidateSizeMB(%d bytes, limit %dMB) (exactly at the ceiling) = %v, want nil", atLimitBytes, limitMB, err),
				}, nil
			}

			return Result{Outcome: OutcomePassed, Detail: fmt.Sprintf("intake.ValidateSizeMB correctly rejects a payload past the %dMB ceiling while still accepting one exactly at it", limitMB)}, nil
		},
	)
}

// ScenarioAuditReplayRejected probes this package's own append-only
// RunRecord persistence (InMemoryRunRecordRepository.Create,
// inmemory_repository.go) for replay resistance: re-submitting the
// exact same RunRecord.ID a second time (the replay abuse case -- an
// attacker or a buggy retrying client resending an already-recorded
// run, attempting to duplicate or silently overwrite audit-relevant
// history with a different reported outcome) must be rejected with
// ErrDuplicateRunRecord, and the original record must remain
// completely unchanged afterward.
func ScenarioAuditReplayRejected() Scenario {
	return NewScenarioFunc(
		"abuse-case/run-record-replay-rejected",
		CategoryAbuseCase,
		func(ctx context.Context) (Result, error) {
			repo := NewInMemoryRunRecordRepository()
			tenantID := uuid.New()

			original := &RunRecord{
				ID:               uuid.New(),
				TenantID:         tenantID,
				ScenarioName:     "adversarial-fixture-replay-target",
				ScenarioCategory: CategoryAbuseCase,
				Result:           Result{Outcome: OutcomePassed, Detail: "original run"},
			}
			if err := repo.Create(ctx, tenantID, original); err != nil {
				return Result{}, fmt.Errorf("seed original RunRecord: %w", err)
			}

			// Replay attempt: an attacker (or a buggy retry) resubmits the
			// identical RunRecord.ID with a mutated Detail, attempting to
			// silently rewrite what the original run reported.
			replay := &RunRecord{
				ID:               original.ID,
				TenantID:         tenantID,
				ScenarioName:     original.ScenarioName,
				ScenarioCategory: original.ScenarioCategory,
				Result:           Result{Outcome: OutcomePassed, Detail: "REPLAYED run claiming a different outcome"},
			}

			err := repo.Create(ctx, tenantID, replay)
			if err == nil {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "Create() with a previously-used RunRecord.ID succeeded instead of being rejected -- replay succeeded",
				}, nil
			}
			if !errors.Is(err, ErrDuplicateRunRecord) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Create() on a replayed RunRecord.ID returned %v, want ErrDuplicateRunRecord", err),
				}, nil
			}

			// The original record must be provably untouched by the
			// rejected replay attempt, not merely "the error was right but
			// the write happened anyway".
			afterReplay, getErr := repo.Get(ctx, tenantID, original.ID)
			if getErr != nil {
				return Result{}, fmt.Errorf("Get(original.ID) after rejected replay attempt: %w", getErr)
			}
			if afterReplay.Result.Detail != original.Result.Detail {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  "Create() returned ErrDuplicateRunRecord but the stored record was mutated anyway -- partial-write replay succeeded",
					Evidence: map[string]string{
						"original_detail": original.Result.Detail,
						"stored_detail":   afterReplay.Result.Detail,
					},
				}, nil
			}
			return Result{Outcome: OutcomePassed, Detail: "InMemoryRunRecordRepository.Create correctly rejects a replayed RunRecord.ID with ErrDuplicateRunRecord and leaves the original record untouched"}, nil
		},
	)
}

// NewAbuseCaseSuite returns every Scenario in this file's fixed
// abuse-case suite.
func NewAbuseCaseSuite() []Scenario {
	return []Scenario{
		ScenarioIntakeDailyQuotaAbuse(),
		ScenarioIntakeConcurrentQuotaAbuse(),
		ScenarioIntakeOversizedPayloadAbuse(),
		ScenarioAuditReplayRejected(),
	}
}
