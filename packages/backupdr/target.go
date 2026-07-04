package backupdr

import (
	"time"

	"github.com/google/uuid"
)

// Target is a DataClass's Recovery Point Objective and Recovery Time
// Objective (task 6): the maximum tolerable data loss window (RPO) and
// the maximum tolerable time to restore service (RTO) for that class
// of data, per tenant.
type Target struct {
	// TenantID is the tenant this target governs.
	TenantID uuid.UUID `json:"tenant_id"`

	// Class is the DataClass this target governs.
	Class DataClass `json:"class"`

	// RPO is the Recovery Point Objective: the maximum acceptable age
	// of a recovery point at the moment of an incident -- how much
	// data, measured in time since the last good backup, this tenant
	// can tolerate losing for this DataClass. Must be positive.
	RPO time.Duration `json:"rpo"`

	// RTO is the Recovery Time Objective: the maximum acceptable
	// duration of a restore operation for this DataClass, from
	// incident declaration to service restored. Must be positive.
	RTO time.Duration `json:"rto"`
}

// Validate checks t for structural well-formedness.
func (t Target) Validate() error {
	if t.TenantID == uuid.Nil {
		return wrapf("Target.Validate", ErrEmptyTenantID)
	}
	if !t.Class.IsValid() {
		return wrapf("Target.Validate", ErrInvalidDataClass)
	}
	if t.RPO <= 0 || t.RTO <= 0 {
		return wrapf("Target.Validate", ErrInvalidTarget)
	}
	return nil
}

// RPOEvaluation is the real-evaluation result EvaluateRPO returns:
// whether a given BackupRecord's age (relative to asOf) satisfies its
// DataClass's registered RPO Target, plus the numbers that decision
// was based on so a caller (dashboard, alert) can explain why.
type RPOEvaluation struct {
	// Record is the BackupRecord being evaluated.
	Record BackupRecord `json:"record"`

	// Target is the RPO/RTO Target this evaluation was run against.
	Target Target `json:"target"`

	// Age is asOf minus Record.TakenAt: how old the backup actually was
	// at the evaluation instant.
	Age time.Duration `json:"age"`

	// Met reports whether Age is within Target.RPO -- i.e. whether this
	// backup, if relied on right now, would satisfy the tenant's
	// tolerated-data-loss window for Record.Class.
	Met bool `json:"met"`
}

// EvaluateRPO evaluates whether record's age as of asOf satisfies
// target's RPO (task 6): real evaluation, not a stub that always
// reports met. Returns ErrInvalidTarget (wrapped) if record.Class !=
// target.Class -- evaluating a record against a mismatched target's
// RPO would silently compare against the wrong data class's tolerance.
func EvaluateRPO(record BackupRecord, target Target, asOf time.Time) (RPOEvaluation, error) {
	if err := target.Validate(); err != nil {
		return RPOEvaluation{}, err
	}
	if record.Class != target.Class {
		return RPOEvaluation{}, wrapf("EvaluateRPO", ErrInvalidTarget)
	}
	age := asOf.Sub(record.TakenAt)
	return RPOEvaluation{
		Record: record,
		Target: target,
		Age:    age,
		Met:    age <= target.RPO,
	}, nil
}

// RTOEvaluation is the real-evaluation result EvaluateRTO returns:
// whether a given RestoreDrill's actual Duration satisfied its
// DataClass's registered RTO Target.
type RTOEvaluation struct {
	// Drill is the RestoreDrill being evaluated.
	Drill RestoreDrill `json:"drill"`

	// Target is the RPO/RTO Target this evaluation was run against.
	Target Target `json:"target"`

	// Met reports whether Drill.Duration was within Target.RTO.
	Met bool `json:"met"`
}

// EvaluateRTO evaluates whether drill's actual Duration satisfies
// target's RTO (task 6): real evaluation over an executed
// RestoreDrill's recorded duration, not a stub. Returns
// ErrInvalidTarget (wrapped) if drill.Class != target.Class.
func EvaluateRTO(drill RestoreDrill, target Target) (RTOEvaluation, error) {
	if err := target.Validate(); err != nil {
		return RTOEvaluation{}, err
	}
	if drill.Class != target.Class {
		return RTOEvaluation{}, wrapf("EvaluateRTO", ErrInvalidTarget)
	}
	return RTOEvaluation{
		Drill:  drill,
		Target: target,
		Met:    drill.Duration <= target.RTO,
	}, nil
}
