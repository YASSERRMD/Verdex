package pilot

import (
	"context"

	"github.com/google/uuid"
)

// DeploymentRepository persists PilotDeployment records, scoped to a
// tenant on every call, mirroring
// packages/vulnmanagement.FindingRepository's conventions.
type DeploymentRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, d *PilotDeployment) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotDeployment, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]PilotDeployment, error)
	Update(ctx context.Context, tenantID uuid.UUID, d *PilotDeployment) error
}

// CaseRepository persists PilotCase records, scoped to a tenant.
type CaseRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, c *PilotCase) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotCase, error)
	ListForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotCase, error)
	Update(ctx context.Context, tenantID uuid.UUID, c *PilotCase) error
}

// FeedbackRepository persists FeedbackEntry records, scoped to a
// tenant.
type FeedbackRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, f *FeedbackEntry) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*FeedbackEntry, error)
	ListForCase(ctx context.Context, tenantID, pilotCaseID uuid.UUID) ([]FeedbackEntry, error)
	ListForDeployment(ctx context.Context, tenantID uuid.UUID, pilotCaseIDs []uuid.UUID) ([]FeedbackEntry, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]FeedbackEntry, error)
}

// FindingRepository persists PilotFinding records, scoped to a tenant.
type FindingRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, f *PilotFinding) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*PilotFinding, error)
	ListForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotFinding, error)
	Update(ctx context.Context, tenantID uuid.UUID, f *PilotFinding) error
}

// RefinementRepository persists RefinementRecord values, scoped to a
// tenant.
type RefinementRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, r *RefinementRecord) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*RefinementRecord, error)
	ListForFinding(ctx context.Context, tenantID, findingID uuid.UUID) ([]RefinementRecord, error)
	ListForDeployment(ctx context.Context, tenantID uuid.UUID, findingIDs []uuid.UUID) ([]RefinementRecord, error)
	Update(ctx context.Context, tenantID uuid.UUID, r *RefinementRecord) error
}
