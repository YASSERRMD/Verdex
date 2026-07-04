package securitytesting

import (
	"context"

	"github.com/google/uuid"
)

// FindingRepository persists Finding records, scoped to a tenant on
// every call (uuid.Nil for a platform-wide finding), mirroring
// packages/compliance.EvidenceRepository's and
// packages/privacy.InventoryRepository's conventions exactly.
type FindingRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, f *Finding) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Finding, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]Finding, error)
	ListByStatus(ctx context.Context, tenantID uuid.UUID, status FindingStatus) ([]Finding, error)
	Update(ctx context.Context, tenantID uuid.UUID, f *Finding) error
}

// RunRecordRepository persists RunRecord values, scoped to a tenant on
// every call (uuid.Nil for a platform-wide run). RunRecords are
// append-only -- there is no Update, mirroring how
// packages/auditlog.Store's own Event records are never mutated after
// Append.
type RunRecordRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, rr *RunRecord) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*RunRecord, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]RunRecord, error)
	ListForScenario(ctx context.Context, tenantID uuid.UUID, scenarioName string) ([]RunRecord, error)
}
