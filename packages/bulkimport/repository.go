package bulkimport

import (
	"context"

	"github.com/google/uuid"
)

// JobRepository persists ImportJob records, scoped to a tenant on
// every call, mirroring packages/privacy.InventoryRepository's and
// packages/compliance.ControlRepository's conventions.
type JobRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, j *ImportJob) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportJob, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]ImportJob, error)
	Update(ctx context.Context, tenantID uuid.UUID, j *ImportJob) error
}

// RecordRepository persists ImportRecord values, scoped to a tenant
// and (for most reads) a specific ImportJob.
type RecordRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *ImportRecord) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ImportRecord, error)

	// ListForJob returns every ImportRecord belonging to jobID, in
	// SourceIndex order.
	ListForJob(ctx context.Context, tenantID, jobID uuid.UUID) ([]ImportRecord, error)

	// FindByDedupKey returns the first ImportRecord within jobID whose
	// DedupKey equals key and whose Outcome is OutcomeImported, used
	// by Engine.RunBatch to detect true duplicates (task 5). Returns
	// ErrRecordNotFound if none exists yet.
	FindByDedupKey(ctx context.Context, tenantID, jobID uuid.UUID, key string) (*ImportRecord, error)

	Update(ctx context.Context, tenantID uuid.UUID, r *ImportRecord) error
}
