package backupdr

import (
	"context"

	"github.com/google/uuid"
)

// PolicyRepository persists BackupPolicy rows, scoped to a tenant on
// every call, mirroring packages/privacy.ConsentRepository's
// conventions. One row exists per tenant/DataClass pair.
type PolicyRepository interface {
	Set(ctx context.Context, tenantID uuid.UUID, p *BackupPolicy) error
	Get(ctx context.Context, tenantID uuid.UUID, class DataClass) (*BackupPolicy, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]BackupPolicy, error)
}

// RecordRepository persists BackupRecord rows, scoped to a tenant.
type RecordRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *BackupRecord) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*BackupRecord, error)
	ListForClass(ctx context.Context, tenantID uuid.UUID, class DataClass) ([]BackupRecord, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]BackupRecord, error)
}

// DrillRepository persists RestoreDrill rows, scoped to a tenant.
type DrillRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, d *RestoreDrill) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*RestoreDrill, error)
	ListForClass(ctx context.Context, tenantID uuid.UUID, class DataClass) ([]RestoreDrill, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]RestoreDrill, error)
}

// TargetRepository persists Target (RPO/RTO) rows, scoped to a tenant.
// One row exists per tenant/DataClass pair, mirroring
// PolicyRepository's shape.
type TargetRepository interface {
	Set(ctx context.Context, tenantID uuid.UUID, t *Target) error
	Get(ctx context.Context, tenantID uuid.UUID, class DataClass) (*Target, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]Target, error)
}
