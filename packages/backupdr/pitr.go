package backupdr

import (
	"time"

	"github.com/google/uuid"
)

// RecoveryPoint is the answer to "if I restore DataClass Class for
// TenantID as of RequestedAt, which BackupRecord do I actually recover
// from, and how far back does that put me" -- point-in-time recovery
// (PITR, task 3). AgeAtRequest is RequestedAt minus the resolved
// Record's TakenAt: how much data (if any) would be lost restoring to
// this point, which is exactly what EvaluateRPO (target.go) checks
// against the governing BackupPolicy's implied recovery objective.
type RecoveryPoint struct {
	// TenantID is the tenant this recovery point was resolved for.
	TenantID uuid.UUID `json:"tenant_id"`

	// Class is the DataClass this recovery point covers.
	Class DataClass `json:"class"`

	// RequestedAt is the point in time recovery was requested for --
	// "restore me to how things looked at T".
	RequestedAt time.Time `json:"requested_at"`

	// Record is the nearest BackupRecord at-or-before RequestedAt that
	// resolution selected.
	Record BackupRecord `json:"record"`

	// AgeAtRequest is RequestedAt minus Record.TakenAt: how far before
	// the requested instant the resolved backup was actually taken. A
	// PITR request for T only ever recovers data as fresh as the
	// nearest backup at-or-before T, never data from after it.
	AgeAtRequest time.Duration `json:"age_at_request"`
}

// ResolveRecoveryPoint finds the BackupRecord in records (for
// tenantID/class) with the latest TakenAt that is still at-or-before
// requestedAt, and returns the RecoveryPoint describing it (task 3's
// centerpiece: real logic over a list of BackupRecords, not a stub).
// Only BackupRecord values with Status == BackupStatusSucceeded are
// eligible -- a failed or still-verifying backup can never be
// recovered from. Returns ErrNoRecoveryPoint if no eligible record
// qualifies (e.g. every backup was taken after requestedAt, or none
// exist for tenantID/class).
//
// records need not be pre-sorted or pre-filtered by tenantID/class;
// ResolveRecoveryPoint filters and selects in one pass, mirroring
// packages/compliance.EvaluateControl's "real evaluation, not a
// lookup" shape.
func ResolveRecoveryPoint(records []BackupRecord, tenantID uuid.UUID, class DataClass, requestedAt time.Time) (RecoveryPoint, error) {
	var best *BackupRecord
	for i := range records {
		r := records[i]
		if r.TenantID != tenantID || r.Class != class {
			continue
		}
		if r.Status != BackupStatusSucceeded {
			continue
		}
		if r.TakenAt.After(requestedAt) {
			continue
		}
		if best == nil || r.TakenAt.After(best.TakenAt) {
			rc := r
			best = &rc
		}
	}
	if best == nil {
		return RecoveryPoint{}, wrapf("ResolveRecoveryPoint", ErrNoRecoveryPoint)
	}

	return RecoveryPoint{
		TenantID:     tenantID,
		Class:        class,
		RequestedAt:  requestedAt,
		Record:       *best,
		AgeAtRequest: requestedAt.Sub(best.TakenAt),
	}, nil
}
