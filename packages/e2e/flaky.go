package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// RetryOnFlake retries fn up to maxAttempts times, returning as soon as
// fn returns a nil error. It exists for exactly one honest purpose:
// guarding against genuinely non-deterministic TIMING (e.g.
// discard_scenario.go's background-goroutine TTL wait racing against a
// loaded CI runner's scheduler), never against a genuine correctness
// defect. RetryOnFlake does not swallow the underlying failure --
// every failed attempt is recorded via t.Logf so a flaky pass is
// visible in test output, not silently hidden, and the final error
// (wrapping ErrMaxAttemptsExhausted) is exactly the last attempt's
// real error if every attempt fails.
//
// maxAttempts must be >= 1; a value <= 0 is treated as 1 (fn always
// runs at least once). delay is the pause between attempts; a zero
// delay retries immediately.
//
// This is a test helper, not a production resilience mechanism: it
// takes *testing.T (or *testing.B via the TB-compatible subset) so its
// use is confined to this package's own _test.go files and any
// downstream caller's tests, never to Scenario.Run itself (a
// Scenario's own Run must report what actually happened on one real
// attempt -- masking a flaky underlying defect behind a retry inside
// Run would defeat the entire suite's purpose).
func RetryOnFlake(t testing.TB, maxAttempts int, delay time.Duration, fn func() error) error {
	t.Helper()

	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			if attempt > 1 {
				t.Logf("RetryOnFlake: succeeded on attempt %d/%d", attempt, maxAttempts)
			}
			return nil
		}
		t.Logf("RetryOnFlake: attempt %d/%d failed: %v", attempt, maxAttempts, lastErr)
		if attempt < maxAttempts && delay > 0 {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("%w: %d attempts, last error: %v", ErrMaxAttemptsExhausted, maxAttempts, lastErr)
}

// QuarantineReason documents why a scenario name is quarantined:
// required so a quarantine entry can never be added silently without
// an accompanying, reviewable justification.
type QuarantineReason struct {
	// ScenarioName must exactly match a Scenario.Name() this suite
	// registers.
	ScenarioName string

	// Reason is a short, honest explanation of the specific
	// non-deterministic timing behavior being guarded against (e.g.
	// "background TTL discard goroutine races CI runner scheduling
	// under load"). A quarantine entry with a blank Reason is rejected
	// by QuarantineList.Validate -- this list exists to guard against
	// genuinely non-deterministic timing, not to silently hide a real,
	// reproducible failure.
	Reason string
}

// QuarantineList is the small, explicit, reviewable set of scenario
// names this suite currently quarantines (task 9). An empty
// QuarantineList (the zero value) is the default and expected steady
// state: every scenario in this package is deterministic as written
// (see each scenario file's own doc comment), so nothing is
// quarantined today. QuarantineList exists as the honest mechanism a
// future genuinely-flaky scenario would be added to, with a required
// Reason, rather than silently skipped or deleted.
type QuarantineList []QuarantineReason

// Validate reports an error if any entry has a blank ScenarioName or
// Reason -- a quarantine entry must always name what it quarantines
// and why.
func (q QuarantineList) Validate() error {
	for _, entry := range q {
		if entry.ScenarioName == "" {
			return wrapf("QuarantineList.Validate", ErrEmptyScenarioName)
		}
		if entry.Reason == "" {
			return fmt.Errorf("e2e: QuarantineList.Validate: scenario %q has a blank Reason", entry.ScenarioName)
		}
	}
	return nil
}

// IsQuarantined reports whether name is present in q.
func (q QuarantineList) IsQuarantined(name string) bool {
	for _, entry := range q {
		if entry.ScenarioName == name {
			return true
		}
	}
	return false
}

// ActiveQuarantineList is this suite's current, real quarantine list.
// It is empty by design (see QuarantineList's doc comment): every
// scenario this package ships is deterministic as written. A future
// contributor discovering genuine flakiness in a specific scenario
// should add an entry here with an honest Reason, not silently retry
// or delete the failing assertion.
var ActiveQuarantineList = QuarantineList{}

// RunAllSkippingQuarantined runs every scenario in s against ctx via
// RunAll, except any scenario name present in quarantine, which is
// recorded as its own SuiteRecord with Result.Outcome == OutcomeErrored
// and a Detail naming ErrQuarantinedScenario plus the quarantine
// Reason -- a quarantined scenario is still visible in the returned
// records (never silently dropped, mirroring Suite.RunAll's own
// "always len(scenarios) records" guarantee), just not executed.
func RunAllSkippingQuarantined(ctx context.Context, s *Suite, quarantine QuarantineList) []SuiteRecord {
	scenarios := s.Scenarios()
	out := make([]SuiteRecord, 0, len(scenarios))
	for _, sc := range scenarios {
		if quarantine.IsQuarantined(sc.Name()) {
			out = append(out, quarantinedRecord(sc, quarantine))
			continue
		}
		record, ok := s.RunOne(ctx, sc.Name())
		if !ok {
			// Unreachable in practice (sc came from s.Scenarios()
			// itself), but handled explicitly rather than silently
			// dropping a record so len(out) == len(scenarios) always
			// holds, matching Suite.RunAll's own contract.
			record = SuiteRecord{ScenarioName: sc.Name(), Result: ScenarioResult{Outcome: OutcomeErrored, Detail: "scenario disappeared between listing and running"}}
		}
		out = append(out, record)
	}
	return out
}

func quarantinedRecord(sc Scenario, quarantine QuarantineList) SuiteRecord {
	reason := ""
	for _, entry := range quarantine {
		if entry.ScenarioName == sc.Name() {
			reason = entry.Reason
			break
		}
	}
	return SuiteRecord{
		ScenarioName:     sc.Name(),
		ScenarioCategory: string(sc.CaseCategory()),
		Result: ScenarioResult{
			Outcome: OutcomeErrored,
			Detail:  fmt.Sprintf("%v: %s", ErrQuarantinedScenario, reason),
		},
		Attempts: 0,
		RanAt:    time.Now().UTC(),
	}
}
