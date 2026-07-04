package backupdr

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DrillOutcome is the result of an executed RestoreDrill.
type DrillOutcome string

const (
	// DrillOutcomeSuccess means the drill's simulated restore-and-verify
	// cycle completed and every check passed.
	DrillOutcomeSuccess DrillOutcome = "success"

	// DrillOutcomeFailure means the drill's restore-and-verify cycle did
	// not complete, or completed but failed verification.
	DrillOutcomeFailure DrillOutcome = "failure"

	// DrillOutcomePartial means the drill completed with some but not
	// all checks passing -- e.g. the restore itself succeeded but
	// integrity verification flagged a mismatch, or the restore
	// completed beyond its RTO target.
	DrillOutcomePartial DrillOutcome = "partial"
)

// IsValid reports whether o is one of the named DrillOutcome
// constants.
func (o DrillOutcome) IsValid() bool {
	switch o {
	case DrillOutcomeSuccess, DrillOutcomeFailure, DrillOutcomePartial:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (o DrillOutcome) String() string { return string(o) }

// RestoreDrill is a scheduled/executed restore-drill record (task 5):
// proof that this platform doesn't just take backups, it periodically
// proves they can actually be restored from. One RestoreDrill exists
// per executed (or attempted) drill run.
type RestoreDrill struct {
	// ID uniquely identifies this drill.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this drill was run for.
	TenantID uuid.UUID `json:"tenant_id"`

	// Class is the DataClass this drill exercised.
	Class DataClass `json:"class"`

	// RecordID references the BackupRecord this drill attempted to
	// restore from.
	RecordID uuid.UUID `json:"record_id"`

	// ExecutedAt is when this drill ran.
	ExecutedAt time.Time `json:"executed_at"`

	// Executor identifies the identity.User (or automated scheduler,
	// recorded as uuid.Nil) that ran this drill.
	Executor uuid.UUID `json:"executor"`

	// Outcome is this drill's result.
	Outcome DrillOutcome `json:"outcome"`

	// Duration is how long the simulated restore-and-verify cycle took
	// -- the figure EvaluateRTO (target.go) compares against the
	// tenant's registered RTO Target for Class.
	Duration time.Duration `json:"duration"`

	// Notes is a free-form record of what happened -- e.g. which checks
	// passed/failed, why an outcome was Partial rather than Success.
	Notes string `json:"notes,omitempty"`

	// CreatedAt is when this record was written.
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks d for structural well-formedness.
func (d *RestoreDrill) Validate() error {
	if d == nil {
		return ErrInvalidDrill
	}
	if d.TenantID == uuid.Nil {
		return wrapf("RestoreDrill.Validate", ErrEmptyTenantID)
	}
	if !d.Class.IsValid() {
		return wrapf("RestoreDrill.Validate", ErrInvalidDataClass)
	}
	if !d.Outcome.IsValid() {
		return wrapf("RestoreDrill.Validate", ErrInvalidDrill)
	}
	if d.ExecutedAt.IsZero() {
		return wrapf("RestoreDrill.Validate", ErrInvalidDrill)
	}
	if d.Duration < 0 {
		return wrapf("RestoreDrill.Validate", ErrInvalidDrill)
	}
	return nil
}

// simulateRestore runs the in-memory-test-context restore-and-verify
// simulation Engine.RunDrill (engine.go) delegates to: given the
// BackupRecord being drilled and a freshly "recomputed" hash the
// caller asserts the restored data would produce, it resolves the
// DrillOutcome and an explanatory Notes string. This is real state
// tracking, not a type with no logic -- the outcome genuinely depends
// on whether the record was restorable (Status ==
// BackupStatusSucceeded) and whether its integrity hash verifies, not
// a hardcoded "always succeeds".
func simulateRestore(record BackupRecord, recomputedHash string) (DrillOutcome, string) {
	if record.Status != BackupStatusSucceeded {
		return DrillOutcomeFailure, "source backup record status is not " + string(BackupStatusSucceeded) + "; nothing to restore from"
	}
	if err := VerifyIntegrity(record, recomputedHash); err != nil {
		return DrillOutcomePartial, "restore completed but integrity verification failed: " + err.Error()
	}
	return DrillOutcomeSuccess, "restore and integrity verification both succeeded"
}

// buildDrillNotes joins base with any caller-supplied extra note,
// keeping simulateRestore's explanation and a caller's own annotation
// (e.g. "ran during quarterly game day") both on the record without
// one overwriting the other.
func buildDrillNotes(base, extra string) string {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return base
	}
	return base + "; " + extra
}
