package backupdr

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Engine is the backup/DR orchestrator: it composes the per-tenant
// BackupPolicy, BackupRecord, RestoreDrill, and Target repositories
// into one set of tenant- and permission-scoped operations, recording
// every policy set, backup, drill, and target change via AuditSink.
// Engine mirrors packages/compliance.Engine and packages/privacy.Engine's
// shape closely: authenticate, check tenant match, check permission,
// mutate, audit regardless of outcome.
type Engine struct {
	policies PolicyRepository
	records  RecordRepository
	drills   DrillRepository
	targets  TargetRepository
	audit    *AuditSink
	clock    func() time.Time
}

// NewEngine builds an Engine from its dependencies. policies, records,
// drills, and targets must be non-nil (ErrNilStore); audit may be nil
// (a nil audit sink means operations simply skip audit recording --
// useful for lightweight unit tests of the decision logic itself,
// though production callers should always supply one).
func NewEngine(policies PolicyRepository, records RecordRepository, drills DrillRepository, targets TargetRepository, audit *AuditSink) (*Engine, error) {
	if policies == nil || records == nil || drills == nil || targets == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		policies: policies,
		records:  records,
		drills:   drills,
		targets:  targets,
		audit:    audit,
		clock:    time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// SetPolicy sets (or replaces) tenantID's BackupPolicy for
// policy.Class (task 1), requiring managePermission and tenant match.
// Every set is recorded via AuditSink regardless of outcome.
func (e *Engine) SetPolicy(ctx context.Context, tenantID uuid.UUID, policy BackupPolicy) (BackupPolicy, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordPolicySet(ctx, tenantID, actorFromCtx(ctx), policy, err)
		}
		return BackupPolicy{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordPolicySet(ctx, tenantID, user.ID, policy, err)
		}
		return BackupPolicy{}, err
	}

	policy.TenantID = tenantID
	if policy.CreatedBy == uuid.Nil {
		policy.CreatedBy = user.ID
	}
	now := e.now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	if err := policy.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordPolicySet(ctx, tenantID, user.ID, policy, err)
		}
		return BackupPolicy{}, err
	}
	if err := e.policies.Set(ctx, tenantID, &policy); err != nil {
		wrapped := wrapf("SetPolicy", err)
		if e.audit != nil {
			_, _ = e.audit.RecordPolicySet(ctx, tenantID, user.ID, policy, wrapped)
		}
		return BackupPolicy{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordPolicySet(ctx, tenantID, user.ID, policy, nil)
	}
	return policy, nil
}

// PolicyFor returns tenantID's currently registered BackupPolicy for
// class, requiring viewPermission and tenant match.
func (e *Engine) PolicyFor(ctx context.Context, tenantID uuid.UUID, class DataClass) (BackupPolicy, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return BackupPolicy{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return BackupPolicy{}, err
	}
	p, err := e.policies.Get(ctx, tenantID, class)
	if err != nil {
		return BackupPolicy{}, wrapf("PolicyFor", err)
	}
	return *p, nil
}

// ListPolicies returns every BackupPolicy registered for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]BackupPolicy, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.policies.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListPolicies", err)
	}
	return list, nil
}

// RecordBackup creates a BackupRecord for tenantID (task 2), requiring
// managePermission and tenant match. Every recorded backup is audited
// regardless of outcome.
func (e *Engine) RecordBackup(ctx context.Context, tenantID uuid.UUID, rec BackupRecord) (BackupRecord, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordBackup(ctx, tenantID, actorFromCtx(ctx), rec, err)
		}
		return BackupRecord{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordBackup(ctx, tenantID, user.ID, rec, err)
		}
		return BackupRecord{}, err
	}

	rec.TenantID = tenantID
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	if rec.CreatedBy == uuid.Nil {
		rec.CreatedBy = user.ID
	}
	if rec.TakenAt.IsZero() {
		rec.TakenAt = e.now()
	}
	rec.CreatedAt = e.now()

	if err := rec.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordBackup(ctx, tenantID, user.ID, rec, err)
		}
		return BackupRecord{}, err
	}
	if err := e.records.Create(ctx, tenantID, &rec); err != nil {
		wrapped := wrapf("RecordBackup", err)
		if e.audit != nil {
			_, _ = e.audit.RecordBackup(ctx, tenantID, user.ID, rec, wrapped)
		}
		return BackupRecord{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordBackup(ctx, tenantID, user.ID, rec, nil)
	}
	return rec, nil
}

// ListBackupRecords returns every BackupRecord on file for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) ListBackupRecords(ctx context.Context, tenantID uuid.UUID) ([]BackupRecord, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.records.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListBackupRecords", err)
	}
	return list, nil
}

// FindRecoveryPoint resolves the nearest recovery point for tenantID's
// class at-or-before requestedAt (task 3's engine-level entry point),
// requiring viewPermission and tenant match. Delegates the actual
// selection logic to the package-level ResolveRecoveryPoint.
func (e *Engine) FindRecoveryPoint(ctx context.Context, tenantID uuid.UUID, class DataClass, requestedAt time.Time) (RecoveryPoint, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return RecoveryPoint{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RecoveryPoint{}, err
	}
	records, err := e.records.ListForClass(ctx, tenantID, class)
	if err != nil {
		return RecoveryPoint{}, wrapf("FindRecoveryPoint", err)
	}
	point, err := ResolveRecoveryPoint(records, tenantID, class, requestedAt)
	if err != nil {
		return RecoveryPoint{}, wrapf("FindRecoveryPoint", err)
	}
	return point, nil
}

// SetTarget sets (or replaces) tenantID's RPO/RTO Target for
// target.Class (task 6), requiring managePermission and tenant match.
func (e *Engine) SetTarget(ctx context.Context, tenantID uuid.UUID, target Target) (Target, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordTargetSet(ctx, tenantID, actorFromCtx(ctx), target, err)
		}
		return Target{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordTargetSet(ctx, tenantID, user.ID, target, err)
		}
		return Target{}, err
	}

	target.TenantID = tenantID
	if err := target.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordTargetSet(ctx, tenantID, user.ID, target, err)
		}
		return Target{}, err
	}
	if err := e.targets.Set(ctx, tenantID, &target); err != nil {
		wrapped := wrapf("SetTarget", err)
		if e.audit != nil {
			_, _ = e.audit.RecordTargetSet(ctx, tenantID, user.ID, target, wrapped)
		}
		return Target{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordTargetSet(ctx, tenantID, user.ID, target, nil)
	}
	return target, nil
}

// TargetFor returns tenantID's currently registered Target for class,
// requiring viewPermission and tenant match.
func (e *Engine) TargetFor(ctx context.Context, tenantID uuid.UUID, class DataClass) (Target, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return Target{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Target{}, err
	}
	t, err := e.targets.Get(ctx, tenantID, class)
	if err != nil {
		return Target{}, wrapf("TargetFor", err)
	}
	return *t, nil
}

// CheckRPO resolves tenantID's registered Target for record.Class and
// evaluates record's age as of now via EvaluateRPO (task 6's
// engine-level entry point), requiring viewPermission and tenant
// match. Returns ErrTargetNotFound if no Target is registered for the
// class.
func (e *Engine) CheckRPO(ctx context.Context, tenantID uuid.UUID, record BackupRecord) (RPOEvaluation, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return RPOEvaluation{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RPOEvaluation{}, err
	}
	target, err := e.targets.Get(ctx, tenantID, record.Class)
	if err != nil {
		return RPOEvaluation{}, wrapf("CheckRPO", err)
	}
	eval, err := EvaluateRPO(record, *target, e.now())
	if err != nil {
		return RPOEvaluation{}, wrapf("CheckRPO", err)
	}
	return eval, nil
}

// RunDrill executes a RestoreDrill for tenantID against recordID (task
// 5's engine-level entry point): resolves the source BackupRecord,
// simulates the restore-and-verify cycle via simulateRestore, and
// persists the resulting RestoreDrill -- real state tracking, not a
// stub that always records success. Requires managePermission and
// tenant match. recomputedHash is the hash the caller asserts the
// restored data produces (see ComputeIntegrityHash); in this
// in-memory-test context there is no real backup blob to read bytes
// from, so callers (including tests) supply the hash a genuine restore
// step would have computed.
func (e *Engine) RunDrill(ctx context.Context, tenantID uuid.UUID, recordID uuid.UUID, recomputedHash string, notes string) (RestoreDrill, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return RestoreDrill{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RestoreDrill{}, err
	}

	record, err := e.records.Get(ctx, tenantID, recordID)
	if err != nil {
		return RestoreDrill{}, wrapf("RunDrill", err)
	}

	start := e.now()
	outcome, base := simulateRestore(*record, recomputedHash)
	finished := e.now()

	drill := RestoreDrill{
		ID:         uuid.New(),
		TenantID:   tenantID,
		Class:      record.Class,
		RecordID:   record.ID,
		ExecutedAt: start,
		Executor:   user.ID,
		Outcome:    outcome,
		Duration:   finished.Sub(start),
		Notes:      buildDrillNotes(base, notes),
		CreatedAt:  finished,
	}

	if err := drill.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordDrill(ctx, tenantID, user.ID, drill, err)
		}
		return RestoreDrill{}, err
	}
	if err := e.drills.Create(ctx, tenantID, &drill); err != nil {
		wrapped := wrapf("RunDrill", err)
		if e.audit != nil {
			_, _ = e.audit.RecordDrill(ctx, tenantID, user.ID, drill, wrapped)
		}
		return RestoreDrill{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordDrill(ctx, tenantID, user.ID, drill, nil)
	}
	return drill, nil
}

// ListDrills returns every RestoreDrill on file for tenantID, requiring
// viewPermission and tenant match.
func (e *Engine) ListDrills(ctx context.Context, tenantID uuid.UUID) ([]RestoreDrill, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.drills.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListDrills", err)
	}
	return list, nil
}

// CheckRTO resolves tenantID's registered Target for drill.Class and
// evaluates drill's actual Duration via EvaluateRTO (task 6's
// engine-level entry point), requiring viewPermission and tenant
// match. Returns ErrTargetNotFound if no Target is registered for the
// class.
func (e *Engine) CheckRTO(ctx context.Context, tenantID uuid.UUID, drill RestoreDrill) (RTOEvaluation, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return RTOEvaluation{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return RTOEvaluation{}, err
	}
	target, err := e.targets.Get(ctx, tenantID, drill.Class)
	if err != nil {
		return RTOEvaluation{}, wrapf("CheckRTO", err)
	}
	eval, err := EvaluateRTO(drill, *target)
	if err != nil {
		return RTOEvaluation{}, wrapf("CheckRTO", err)
	}
	return eval, nil
}
