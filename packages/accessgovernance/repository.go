package accessgovernance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PolicyRepository persists Policy records, scoped to a tenant on
// every call, mirroring packages/keymanagement.Repository's
// conventions.
type PolicyRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, p *Policy) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Policy, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]Policy, error)
	Update(ctx context.Context, tenantID uuid.UUID, p *Policy) error
}

// CaseGrantRepository persists CaseGrant records, scoped to a tenant.
type CaseGrantRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, g *CaseGrant) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*CaseGrant, error)
	ListForCase(ctx context.Context, tenantID, caseID uuid.UUID) ([]CaseGrant, error)
	ListActive(ctx context.Context, tenantID uuid.UUID, now time.Time) ([]CaseGrant, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]CaseGrant, error)
	Revoke(ctx context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error
}

// GrantRepository persists Grant (JIT elevation) records, scoped to a
// tenant, distinct from CaseGrantRepository since elevations are keyed
// by Action rather than by a fixed Permissions list and may or may not
// be case-scoped.
type GrantRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, g *Grant) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Grant, error)
	ListActive(ctx context.Context, tenantID uuid.UUID, now time.Time) ([]Grant, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]Grant, error)
	Revoke(ctx context.Context, tenantID, id uuid.UUID, revokedAt time.Time) error
}

// ReviewRepository persists Review records, scoped to a tenant.
type ReviewRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *Review) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*Review, error)
	ListDue(ctx context.Context, tenantID uuid.UUID, asOf time.Time) ([]Review, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]Review, error)
	Update(ctx context.Context, tenantID uuid.UUID, r *Review) error
}
