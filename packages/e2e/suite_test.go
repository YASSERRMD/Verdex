package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/e2e"
)

// fakeScenario is a minimal Scenario implementation for testing Suite
// mechanics in isolation, without needing a real full-journey call.
type fakeScenario struct {
	name   string
	cat    category.CategoryCode
	result e2e.ScenarioResult
	err    error
}

func (f fakeScenario) Name() string                        { return f.name }
func (f fakeScenario) CaseCategory() category.CategoryCode { return f.cat }
func (f fakeScenario) Run(_ context.Context) (e2e.ScenarioResult, error) {
	return f.result, f.err
}

var _ e2e.Scenario = fakeScenario{}

func TestSuite_RunAll_ReturnsOneRecordPerScenario(t *testing.T) {
	suite := e2e.NewSuite(
		fakeScenario{name: "b", cat: category.CodeCivil, result: e2e.ScenarioResult{Outcome: e2e.OutcomePassed}},
		fakeScenario{name: "a", cat: category.CodeCivil, result: e2e.ScenarioResult{Outcome: e2e.OutcomeFailed, Detail: "boom"}},
		fakeScenario{name: "c", cat: category.CodeCivil, err: errors.New("construction failed")},
	)

	records := suite.RunAll(context.Background())
	if len(records) != 3 {
		t.Fatalf("len(records) = %d, want 3", len(records))
	}
	// Scenarios() sorts by Name, so "a" must come first.
	if records[0].ScenarioName != "a" {
		t.Fatalf("records[0].ScenarioName = %q, want %q", records[0].ScenarioName, "a")
	}
	if records[0].Result.Outcome != e2e.OutcomeFailed {
		t.Fatalf("records[0].Result.Outcome = %q, want %q", records[0].Result.Outcome, e2e.OutcomeFailed)
	}
	if records[1].ScenarioName != "b" || records[1].Result.Outcome != e2e.OutcomePassed {
		t.Fatalf("records[1] = %+v, want name=b outcome=passed", records[1])
	}
	if records[2].ScenarioName != "c" || records[2].Result.Outcome != e2e.OutcomeErrored {
		t.Fatalf("records[2] = %+v, want name=c outcome=errored (Run itself returned an error)", records[2])
	}

	if e2e.AllPassed(records) {
		t.Fatalf("AllPassed(records) = true, want false (one failed, one errored)")
	}
	if len(e2e.FailedRecords(records)) != 1 {
		t.Fatalf("len(FailedRecords(records)) = %d, want 1", len(e2e.FailedRecords(records)))
	}
	if len(e2e.ErroredRecords(records)) != 1 {
		t.Fatalf("len(ErroredRecords(records)) = %d, want 1", len(e2e.ErroredRecords(records)))
	}
}

func TestSuite_RunOne_NotFound(t *testing.T) {
	suite := e2e.NewSuite(fakeScenario{name: "only", cat: category.CodeCivil, result: e2e.ScenarioResult{Outcome: e2e.OutcomePassed}})

	if _, ok := suite.RunOne(context.Background(), "does-not-exist"); ok {
		t.Fatalf("RunOne(does-not-exist) ok = true, want false")
	}

	record, ok := suite.RunOne(context.Background(), "only")
	if !ok {
		t.Fatalf("RunOne(only) ok = false, want true")
	}
	if record.Result.Outcome != e2e.OutcomePassed {
		t.Fatalf("record.Result.Outcome = %q, want passed", record.Result.Outcome)
	}
}

func TestSuite_AllPassed_EmptyIsTrue(t *testing.T) {
	if !e2e.AllPassed(nil) {
		t.Fatalf("AllPassed(nil) = false, want true (vacuously all passed)")
	}
}

func TestSuite_InvalidResultBecomesErrored(t *testing.T) {
	// A Scenario returning Outcome=Failed with a blank Detail fails its
	// own ScenarioResult.Validate -- the Suite must catch this and
	// downgrade it to Errored rather than propagating a malformed
	// record, mirroring packages/securitytesting.Harness.runOne's
	// identical malformed-scenario guard.
	suite := e2e.NewSuite(fakeScenario{
		name:   "malformed",
		cat:    category.CodeCivil,
		result: e2e.ScenarioResult{Outcome: e2e.OutcomeFailed, Detail: ""},
	})

	records := suite.RunAll(context.Background())
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].Result.Outcome != e2e.OutcomeErrored {
		t.Fatalf("Result.Outcome = %q, want errored (malformed ScenarioResult)", records[0].Result.Outcome)
	}
}

func TestRetryOnFlake_SucceedsEventually(t *testing.T) {
	attempts := 0
	err := e2e.RetryOnFlake(t, 3, time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("attempt %d: simulated timing flake", attempts)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryOnFlake: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryOnFlake_ExhaustsAttempts(t *testing.T) {
	attempts := 0
	err := e2e.RetryOnFlake(t, 2, time.Millisecond, func() error {
		attempts++
		return errors.New("persistent failure")
	})
	if err == nil {
		t.Fatalf("expected an error after exhausting attempts, got nil")
	}
	if !errors.Is(err, e2e.ErrMaxAttemptsExhausted) {
		t.Fatalf("err = %v, want it to wrap ErrMaxAttemptsExhausted", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (must not retry past maxAttempts)", attempts)
	}
}

func TestRetryOnFlake_FirstAttemptSucceeds(t *testing.T) {
	attempts := 0
	err := e2e.RetryOnFlake(t, 5, 0, func() error {
		attempts++
		return nil
	})
	if err != nil {
		t.Fatalf("RetryOnFlake: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (must not retry when the first attempt succeeds)", attempts)
	}
}

func TestQuarantineList_ValidateRejectsBlankReason(t *testing.T) {
	q := e2e.QuarantineList{{ScenarioName: "some/scenario", Reason: ""}}
	if err := q.Validate(); err == nil {
		t.Fatalf("expected Validate to reject a blank Reason, got nil")
	}
}

func TestQuarantineList_ValidateAcceptsWellFormedEntry(t *testing.T) {
	q := e2e.QuarantineList{{ScenarioName: "some/scenario", Reason: "genuinely non-deterministic timing under CI load"}}
	if err := q.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !q.IsQuarantined("some/scenario") {
		t.Fatalf("IsQuarantined(some/scenario) = false, want true")
	}
	if q.IsQuarantined("other/scenario") {
		t.Fatalf("IsQuarantined(other/scenario) = true, want false")
	}
}

func TestActiveQuarantineList_IsEmptyAndValid(t *testing.T) {
	// This suite's real, active quarantine list must start empty (every
	// scenario in this package is deterministic as written) and must
	// always pass its own Validate.
	if len(e2e.ActiveQuarantineList) != 0 {
		t.Fatalf("len(ActiveQuarantineList) = %d, want 0 -- see flaky.go's doc comment before adding an entry", len(e2e.ActiveQuarantineList))
	}
	if err := e2e.ActiveQuarantineList.Validate(); err != nil {
		t.Fatalf("ActiveQuarantineList.Validate: %v", err)
	}
}

func TestRunAllSkippingQuarantined(t *testing.T) {
	suite := e2e.NewSuite(
		fakeScenario{name: "quarantined", cat: category.CodeCivil, result: e2e.ScenarioResult{Outcome: e2e.OutcomePassed}},
		fakeScenario{name: "normal", cat: category.CodeCivil, result: e2e.ScenarioResult{Outcome: e2e.OutcomePassed}},
	)
	quarantine := e2e.QuarantineList{{ScenarioName: "quarantined", Reason: "test fixture"}}

	records := e2e.RunAllSkippingQuarantined(context.Background(), suite, quarantine)
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2 (quarantined scenario must still produce a record)", len(records))
	}

	var quarantinedRecord, normalRecord *e2e.SuiteRecord
	for i := range records {
		switch records[i].ScenarioName {
		case "quarantined":
			quarantinedRecord = &records[i]
		case "normal":
			normalRecord = &records[i]
		}
	}
	if quarantinedRecord == nil || normalRecord == nil {
		t.Fatalf("expected both quarantined and normal records present, got %+v", records)
	}
	if quarantinedRecord.Result.Outcome != e2e.OutcomeErrored {
		t.Fatalf("quarantined record Outcome = %q, want errored", quarantinedRecord.Result.Outcome)
	}
	if normalRecord.Result.Outcome != e2e.OutcomePassed {
		t.Fatalf("normal record Outcome = %q, want passed (must still run)", normalRecord.Result.Outcome)
	}
}
