package corpusupdater

import (
	"context"

	"github.com/google/uuid"
)

// JobRepository persists CorpusUpdateJob records, scoped to a tenant
// on every call, mirroring packages/compliance.ControlRepository's and
// packages/privacy.InventoryRepository's conventions.
type JobRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*CorpusUpdateJob, error)
	ListByJurisdiction(ctx context.Context, tenantID uuid.UUID, jurisdictionCode string) ([]CorpusUpdateJob, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]CorpusUpdateJob, error)
	Update(ctx context.Context, tenantID uuid.UUID, j *CorpusUpdateJob) error
}

// AmendmentRepository persists Amendment records, scoped to a tenant on
// every call.
type AmendmentRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, a *Amendment) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Amendment, error)
	ListForJob(ctx context.Context, tenantID, jobID uuid.UUID) ([]Amendment, error)
	ListForTarget(ctx context.Context, tenantID uuid.UUID, corpus CorpusTarget, targetID string) ([]Amendment, error)
	Update(ctx context.Context, tenantID uuid.UUID, a *Amendment) error
}
