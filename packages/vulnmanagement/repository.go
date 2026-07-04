package vulnmanagement

import (
	"context"

	"github.com/google/uuid"
)

// FindingRepository persists Finding records, scoped to a tenant on
// every call, mirroring packages/compliance.EvidenceRepository's and
// packages/privacy.InventoryRepository's conventions.
type FindingRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, f *Finding) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Finding, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]Finding, error)
	ListBySource(ctx context.Context, tenantID uuid.UUID, source ScannerSource) ([]Finding, error)
	ListByStatus(ctx context.Context, tenantID uuid.UUID, status Status) ([]Finding, error)
	Update(ctx context.Context, tenantID uuid.UUID, f *Finding) error
}

// TriageRepository persists TriageDecision records, scoped to a
// tenant on every call.
type TriageRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, d *TriageDecision) error
	ListForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]TriageDecision, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]TriageDecision, error)
}
