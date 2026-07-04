package privacy

import (
	"context"

	"github.com/google/uuid"
)

// InventoryRepository persists DataInventoryEntry records, scoped to a
// tenant on every call, mirroring
// packages/accessgovernance.PolicyRepository's conventions.
type InventoryRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*DataInventoryEntry, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]DataInventoryEntry, error)
	Update(ctx context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error
}

// ConsentRepository persists ConsentRecord values, scoped to a tenant.
type ConsentRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, c *ConsentRecord) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ConsentRecord, error)
	ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ConsentRecord, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]ConsentRecord, error)
	Update(ctx context.Context, tenantID uuid.UUID, c *ConsentRecord) error
}

// SARRepository persists SubjectAccessRequest values, scoped to a
// tenant.
type SARRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *SubjectAccessRequest) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*SubjectAccessRequest, error)
	ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]SubjectAccessRequest, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]SubjectAccessRequest, error)
	Update(ctx context.Context, tenantID uuid.UUID, r *SubjectAccessRequest) error
}

// ErasureRepository persists ErasureRequest values, scoped to a
// tenant.
type ErasureRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *ErasureRequest) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ErasureRequest, error)
	ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ErasureRequest, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]ErasureRequest, error)
	Update(ctx context.Context, tenantID uuid.UUID, r *ErasureRequest) error
}
